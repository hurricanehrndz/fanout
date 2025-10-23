package fanout

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestTCPRetryOnTruncatedUDP(t *testing.T) {
	udpCallCount := 0
	tcpCallCount := 0

	s := newServer(UDP, func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)

		if w.RemoteAddr().Network() == UDP {
			udpCallCount++
			msg.Truncated = true
			msg.Answer = []dns.RR{
				makeRecordA("example.com. 3600 IN A 10.0.0.1"),
			}
		} else {
			tcpCallCount++
			msg.Truncated = false
			msg.Answer = []dns.RR{
				makeRecordA("example.com. 3600 IN A 10.0.0.1"),
				makeRecordA("example.com. 3600 IN A 10.0.0.2"),
			}
		}
		logErrIfNotNil(w.WriteMsg(&msg))
	})
	defer s.close()

	c := NewClient(s.addr, UDP)
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	resp, err := c.Request(ctx, &request.Request{W: &test.ResponseWriter{}, Req: req})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 1, udpCallCount)
	require.Equal(t, 1, tcpCallCount)
	require.Len(t, resp.Answer, 2)
}
