# fanout
[![CI](https://github.com/hurricanehrndz/fanout/actions/workflows/ci.yaml/badge.svg?event=pull_request)](https://github.com/hurricanehrndz/fanout/actions/workflows/ci.yaml)
## Name

*fanout* - parallel proxying DNS messages to upstream resolvers.

## Description

Each incoming DNS query that matches the CoreDNS fanout plugin is sent concurrently to the selected upstream resolvers. Without `race`, the first valid answer-bearing NOERROR response is forwarded; NODATA and negative responses are retained as fallbacks while waiting.

## Syntax

* `tls` **CERT** **KEY** **CA** define the TLS properties for TLS connection. From 0 to 3 arguments can be
  provided with the meaning as described below
  * `tls` - no client authentication is used, and the system CAs are used to verify the server certificate
  * `tls` **CA** - no client authentication is used, and the file CA is used to verify the server certificate
  * `tls` **CERT** **KEY** - client authentication is used with the specified cert/key pair.
    The server certificate is verified with the system CAs
  * `tls` **CERT** **KEY**  **CA** - client authentication is used with the specified cert/key pair.
    The server certificate is verified using the specified CA file
* `tls-server` **NAME** allows you to set a server name in the TLS configuration; for instance 9.9.9.9
  needs this to be set to `dns.quad9.net`. Multiple upstreams are still allowed in this scenario,
  but they have to use the same `tls-server`. E.g. mixing 9.9.9.9 (QuadDNS) with 1.1.1.1
  (Cloudflare) will not work.

* `worker-count` is the number of parallel queries per request. By default equals to count of IP list. Use this only for reducing parallel queries per request.
* `policy` - specifies the policy of DNS server selection mechanism. The default is `sequential`.
  * `sequential` - select DNS servers one-by-one based on its order
  * `weighted-random` - select DNS servers randomly based on `weighted-random-server-count` and `weighted-random-load-factor` params.
* `weighted-random-server-count` is the number of DNS servers to be requested. Equals to the number of specified IPs by default. Used only with the `weighted-random` policy.
* `weighted-random-load-factor` - the probability of selecting a server. This is specified in the order of the list of IP addresses and takes values between 1 and 100. By default, all servers have an equal probability of 100. Used only with the `weighted-random` policy.
* `network` is the upstream network protocol: `tcp`, `udp`, or `tcp-tls`. UDP responses with the truncated flag set are retried over TCP automatically.
* `except` is a space-separated list of domains to exclude from proxying.
* `except-file` is the path to a file containing one excluded domain per line.
* `attempt-count` is the number of attempts per selected upstream before returning its error. If `0`, attempts continue until `timeout`. Default is `3`.
* `timeout` is the overall request timeout. After this period, attempts to receive a response from the upstream servers stop. Default is `30s`.
* `udp-buffer-size` overrides the UDP buffer size advertised in EDNS0 requests to upstream servers. Minimum value is `1232` bytes (RFC 6891). When omitted, existing EDNS0 is preserved and requests without EDNS0 advertise `1232`. This setting only affects UDP queries; TCP queries are unaffected. Should only be used with local resolvers.
* `race` returns the first valid DNS result, including NODATA or a negative response, instead of waiting for an answer-bearing NOERROR response.
* `next` **RCODE...** delegates to the next `fanout` stanza when the result has one of the listed DNS response codes, such as `NXDOMAIN` or `SERVFAIL`. It is ignored when the next handler is not another `fanout` stanza.

## Metadata

If the *metadata* plugin is enabled, `fanout/upstream` contains the upstream that supplied the response. If the *dnstap* plugin is enabled, fanout emits the selected upstream query and response.

## Metrics

If monitoring is enabled (via the *prometheus* plugin) then the following metric are exported:

* `coredns_fanout_request_duration_seconds{to}` - duration per upstream interaction.
* `coredns_fanout_request_count_total{to}` - query count per upstream.
* `coredns_fanout_response_rcode_count_total{to, rcode}` - count of RCODEs per upstream.

Where `to` is one of the upstream servers (**TO** from the config), `rcode` is the returned RCODE
from the upstream.

## Examples
Proxy all requests within `example.org.` to a nameservers running on a different ports.  The first positive response from a proxy will be provided as the result.

~~~ corefile
example.org {
    fanout . 127.0.0.1:9005 127.0.0.1:9006 127.0.0.1:9007 127.0.0.1:9008
}
~~~

Sends parallel requests between three resolvers, one of which has an IPv6 address, via TCP. The first positive response from a proxy will be provided as the result.

~~~ corefile
. {
    fanout . 10.0.0.10:53 10.0.0.11:1053 [2003::1]:53 {
        network TCP
    }
}
~~~

Proxying everything except requests to `example.org`

~~~ corefile
. {
    fanout . 10.0.0.10:1234 {
        except example.org
    }
}
~~~

Proxy everything except `example.org` using the host's `resolv.conf`'s nameservers:

~~~ corefile
. {
    fanout . /etc/resolv.conf {
        except example.org
    }
}
~~~

Proxy all requests to 9.9.9.9 using the DNS-over-TLS protocol.
Note the `tls-server` is mandatory if you want a working setup, as 9.9.9.9 can't be
used in the TLS negotiation.

~~~ corefile
. {
    fanout . tls://9.9.9.9 {
       tls-server dns.quad9.net
    }
}
~~~

Sends parallel requests between five resolvers via UDP using two workers. The first positive response from a proxy will be provided as the result.
~~~ corefile
. {
    fanout . 10.0.0.10:53 10.0.0.11:53 10.0.0.12:53 10.0.0.13:1053 10.0.0.14:1053 {
        worker-count 2
    }
}
~~~

Multiple upstream servers are configured but one of them is down while querying a `non-existent` domain.
With `race` enabled, the first valid `NXDOMAIN` response is returned immediately. Otherwise, fanout retains it as a fallback while the remaining selected upstreams complete or time out.
~~~ corefile
. {
    fanout . 10.0.0.10:53 10.0.0.11:53 10.0.0.12:53 10.0.0.13:1053 10.0.0.14:1053 {
        race
    }
}
~~~

Chain two fanout configurations, using the second when the first returns `NXDOMAIN` or `SERVFAIL`.
~~~ corefile
. {
    fanout . 10.0.0.10:53 10.0.0.11:53 {
        next NXDOMAIN SERVFAIL
    }
    fanout . 192.0.2.10:53 192.0.2.11:53
}
~~~

Sends parallel requests between two randomly selected resolvers. Note that `127.0.0.1:9007` is selected more frequently because it has the highest `weighted-random-load-factor`.
~~~ corefile
example.org {
    fanout . 127.0.0.1:9005 127.0.0.1:9006 127.0.0.1:9007 {
      policy weighted-random
      weighted-random-server-count 2
      weighted-random-load-factor 50 70 100
    }
}
~~~

Sends parallel requests between three resolver sequentially (default mode).
~~~ corefile
example.org {
    fanout . 127.0.0.1:9005 127.0.0.1:9006 127.0.0.1:9007 {
        policy sequential
    }
}
~~~

Use a larger UDP buffer size for upstream queries. This can help prevent truncation for large responses.
~~~ corefile
. {
    fanout . 8.8.8.8 8.8.4.4 {
        udp-buffer-size 4096
    }
}
~~~
