package fanout

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestTCPRetryOnTruncatedUDP(t *testing.T) {
	var udpCallCount, tcpCallCount atomic.Int32

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)

		network := w.RemoteAddr().Network()

		if network == UDP {
			udpCallCount.Add(1)
			msg.Truncated = true
			msg.Answer = []dns.RR{
				makeRecordA("example.com. 3600 IN A 10.0.0.1"),
			}
		} else {
			tcpCallCount.Add(1)
			msg.Truncated = false
			msg.Answer = []dns.RR{
				makeRecordA("example.com. 3600 IN A 10.0.0.1"),
				makeRecordA("example.com. 3600 IN A 10.0.0.2"),
			}
		}
		logErrIfNotNil(w.WriteMsg(&msg))
	}

	tcpListener, err := net.Listen(TCP, "127.0.0.1:0")
	require.NoError(t, err)
	defer tcpListener.Close()

	udpConn, err := net.ListenPacket("udp", tcpListener.Addr().String())
	require.NoError(t, err)
	defer udpConn.Close()

	tcpServer := &dns.Server{Listener: tcpListener, Handler: dns.HandlerFunc(handler)}
	udpServer := &dns.Server{PacketConn: udpConn, Handler: dns.HandlerFunc(handler)}

	go func() { _ = tcpServer.ActivateAndServe() }()
	go func() { _ = udpServer.ActivateAndServe() }()
	defer tcpServer.Shutdown()
	defer udpServer.Shutdown()

	c := NewClient(tcpListener.Addr().String(), UDP)
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	resp, err := c.Request(ctx, &request.Request{W: &test.ResponseWriter{}, Req: req})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated, "TCP response should not be truncated")
	require.Equal(t, int32(1), udpCallCount.Load(), "Expected exactly 1 UDP call")
	require.Equal(t, int32(1), tcpCallCount.Load(), "Expected exactly 1 TCP call")
	require.Len(t, resp.Answer, 2, "TCP response should have 2 answers")
}
