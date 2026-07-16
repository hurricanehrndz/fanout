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
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestClientAdvertisesConfiguredUDPBufferSizeWithoutMutatingRequest(t *testing.T) {
	tests := []struct {
		name       string
		configured uint16
		withOPT    bool
	}{
		{name: "replace existing OPT", configured: 4096, withOPT: true},
		{name: "add absent OPT", configured: 65535},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan *dns.Msg, 1)
			s := newServer(UDP, func(w dns.ResponseWriter, req *dns.Msg) {
				received <- req.Copy()
				resp := new(dns.Msg)
				resp.SetReply(req)
				logErrIfNotNil(w.WriteMsg(resp))
			})
			defer s.close()

			req := new(dns.Msg)
			req.SetQuestion(testQuery, dns.TypeA)
			if tc.withOPT {
				req.SetEdns0(1232, true)
				req.IsEdns0().Option = append(req.IsEdns0().Option, &dns.EDNS0_LOCAL{
					Code: dns.EDNS0LOCALSTART,
					Data: []byte{6, 1},
				})
			}
			original, err := req.Pack()
			require.NoError(t, err)

			c := NewClientWithUDPBufferSize(s.addr, UDP, tc.configured)
			c.(*client).udpBufferSizeOverride = tc.configured
			_, err = c.Request(context.Background(), &request.Request{W: &test.ResponseWriter{}, Req: req})
			require.NoError(t, err)

			wireReq := <-received
			require.Equal(t, tc.configured, wireReq.IsEdns0().UDPSize())
			if tc.withOPT {
				require.True(t, wireReq.IsEdns0().Do())
				require.Equal(t, req.IsEdns0().Option, wireReq.IsEdns0().Option)
			}
			after, err := req.Pack()
			require.NoError(t, err)
			require.Equal(t, original, after)
		})
	}
}

func TestClientPreservesIncomingEDNSWithoutUDPBufferOverride(t *testing.T) {
	tests := []struct {
		name     string
		withOPT  bool
		expected uint16
	}{
		{name: "preserve existing OPT", withOPT: true, expected: 65535},
		{name: "add absent OPT with client minimum", expected: 4096},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan *dns.Msg, 1)
			s := newServer(UDP, func(w dns.ResponseWriter, req *dns.Msg) {
				received <- req.Copy()
				resp := new(dns.Msg)
				resp.SetReply(req)
				logErrIfNotNil(w.WriteMsg(resp))
			})
			defer s.close()

			req := new(dns.Msg)
			req.SetQuestion(testQuery, dns.TypeA)
			if tc.withOPT {
				req.SetEdns0(tc.expected, true)
				req.IsEdns0().Option = append(req.IsEdns0().Option, &dns.EDNS0_LOCAL{
					Code: dns.EDNS0LOCALSTART,
					Data: []byte{6, 1},
				})
			}
			original, err := req.Pack()
			require.NoError(t, err)

			c := NewClientWithUDPBufferSize(s.addr, UDP, 4096)
			_, err = c.Request(context.Background(), &request.Request{W: &test.ResponseWriter{}, Req: req})
			require.NoError(t, err)

			wireReq := <-received
			require.Equal(t, tc.expected, wireReq.IsEdns0().UDPSize())
			if tc.withOPT {
				require.True(t, wireReq.IsEdns0().Do())
				require.Equal(t, req.IsEdns0().Option, wireReq.IsEdns0().Option)
			}
			after, err := req.Pack()
			require.NoError(t, err)
			require.Equal(t, original, after)
		})
	}
}

func TestClientDoesNotOverrideEDNSForTCP(t *testing.T) {
	tests := []struct {
		name    string
		withOPT bool
	}{
		{name: "preserve existing OPT", withOPT: true},
		{name: "do not add absent OPT"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan *dns.Msg, 1)
			s := newServer(TCP, func(w dns.ResponseWriter, req *dns.Msg) {
				received <- req.Copy()
				resp := new(dns.Msg)
				resp.SetReply(req)
				logErrIfNotNil(w.WriteMsg(resp))
			})
			defer s.close()

			req := new(dns.Msg)
			req.SetQuestion(testQuery, dns.TypeA)
			if tc.withOPT {
				req.SetEdns0(1232, true)
				req.IsEdns0().Option = append(req.IsEdns0().Option, &dns.EDNS0_LOCAL{
					Code: dns.EDNS0LOCALSTART,
					Data: []byte{6, 1},
				})
			}
			original, err := req.Pack()
			require.NoError(t, err)

			c := NewClientWithUDPBufferSize(s.addr, TCP, 4096)
			c.(*client).udpBufferSizeOverride = 65535
			_, err = c.Request(context.Background(), &request.Request{W: &test.ResponseWriter{}, Req: req})
			require.NoError(t, err)
			wireReq := <-received
			require.Equal(t, req.Question, wireReq.Question)
			if tc.withOPT {
				require.Equal(t, uint16(1232), wireReq.IsEdns0().UDPSize())
				require.True(t, wireReq.IsEdns0().Do())
				require.Equal(t, req.IsEdns0().Option, wireReq.IsEdns0().Option)
			} else {
				require.Nil(t, wireReq.IsEdns0())
			}
			after, err := req.Pack()
			require.NoError(t, err)
			require.Equal(t, original, after)
		})
	}
}
