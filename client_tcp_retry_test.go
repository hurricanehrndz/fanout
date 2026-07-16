// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fanout

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type fallbackCancellationTransport struct {
	Transport
	tcpDialed chan struct{}
}

func (t *fallbackCancellationTransport) Dial(ctx context.Context, network string) (*dns.Conn, error) {
	conn, err := t.Transport.Dial(ctx, network)
	if network == TCP && err == nil {
		close(t.tcpDialed)
		<-ctx.Done()
	}
	return conn, err
}

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

func TestClientCancellationDuringUDPToTCPFallbackIsRaceFree(t *testing.T) {
	s := newServer(UDP, func(w dns.ResponseWriter, req *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Truncated = w.RemoteAddr().Network() == UDP
		logErrIfNotNil(w.WriteMsg(resp))
	})
	defer s.close()

	for range 50 {
		transport := &fallbackCancellationTransport{
			Transport: NewTransport(s.addr),
			tcpDialed: make(chan struct{}),
		}
		c := &client{addr: s.addr, net: UDP, transport: transport, udpBufferSize: minUDPBufferSize}
		req := new(dns.Msg)
		req.SetQuestion(testQuery, dns.TypeA)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := c.Request(ctx, &request.Request{W: &test.ResponseWriter{}, Req: req})
			result <- err
		}()

		select {
		case <-transport.tcpDialed:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("TCP fallback did not start")
		}
		cancel()
		select {
		case err := <-result:
			require.Error(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("canceled request did not return promptly")
		}
	}
}

func TestClientRequestBackgroundDoesNotLeakWatcher(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := newServer(UDP, func(w dns.ResponseWriter, req *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		logErrIfNotNil(w.WriteMsg(resp))
	})
	defer s.close()

	c := NewClient(s.addr, UDP)
	req := new(dns.Msg)
	req.SetQuestion(testQuery, dns.TypeA)
	_, err := c.Request(context.Background(), &request.Request{W: &test.ResponseWriter{}, Req: req})
	require.NoError(t, err)
}
