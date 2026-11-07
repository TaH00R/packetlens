package probe

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func dnsServer(t *testing.T, reply func(string, dnsmessage.Message) []byte) string {
	t.Helper()
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	udp, err := net.ListenPacket("udp", tcp.Addr().String())
	if err != nil {
		tcp.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { udp.Close(); tcp.Close() })
	go func() {
		packet := make([]byte, 512)
		for {
			n, peer, err := udp.ReadFrom(packet)
			if err != nil {
				return
			}
			var request dnsmessage.Message
			if request.Unpack(packet[:n]) != nil {
				continue
			}
			if response := reply("udp", request); response != nil {
				udp.WriteTo(response, peer)
			}
		}
	}()
	go func() {
		for {
			conn, err := tcp.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(time.Second))
				var prefix [2]byte
				if _, err := io.ReadFull(conn, prefix[:]); err != nil {
					return
				}
				packet := make([]byte, int(binary.BigEndian.Uint16(prefix[:])))
				if _, err := io.ReadFull(conn, packet); err != nil {
					return
				}
				var request dnsmessage.Message
				if request.Unpack(packet) != nil {
					return
				}
				response := reply("tcp", request)
				binary.BigEndian.PutUint16(prefix[:], uint16(len(response)))
				conn.Write(append(prefix[:], response...))
			}()
		}
	}()
	return tcp.Addr().String()
}
func dnsAnswer(q dnsmessage.Message) dnsmessage.Message {
	return dnsmessage.Message{Header: dnsmessage.Header{ID: q.ID, Response: true, RecursionAvailable: true}, Questions: q.Questions, Answers: []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: q.Questions[0].Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60}, Body: &dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}}}}}
}
func TestDirectResolverOutcomesAndTCPFallback(t *testing.T) {
	for _, name := range []string{"success", "fallback", "nxdomain", "empty", "wrong-id", "wrong-question", "malformed", "unrelated-answer", "timeout"} {
		t.Run(name, func(t *testing.T) {
			var udp, tcp atomic.Int32
			endpoint := dnsServer(t, func(network string, q dnsmessage.Message) []byte {
				if network == "udp" {
					udp.Add(1)
				} else {
					tcp.Add(1)
				}
				response := dnsAnswer(q)
				switch name {
				case "fallback":
					if network == "udp" {
						response.Truncated = true
						response.Answers = nil
					}
				case "nxdomain":
					response.RCode = dnsmessage.RCodeNameError
					response.Answers = nil
				case "empty":
					response.Answers = nil
				case "wrong-id":
					response.ID++
				case "wrong-question":
					response.Questions[0].Type = dnsmessage.TypeAAAA
				case "malformed":
					return []byte{1, 2, 3}
				case "unrelated-answer":
					response.Answers[0].Header.Name, _ = dnsmessage.NewName("other.test.")
				case "timeout":
					return nil
				}
				wire, _ := response.Pack()
				return wire
			})
			// localhost intentionally exercises bypass of the OS hosts file.
			r := CheckDNSResolver(context.Background(), "localhost", endpoint, 200*time.Millisecond)
			expected := "success"
			switch name {
			case "nxdomain":
				expected = "dns_error"
			case "empty", "unrelated-answer":
				expected = "no_answer"
			case "wrong-id", "wrong-question", "malformed":
				expected = "invalid_response"
			case "timeout":
				expected = "timeout"
			}
			if r.Outcome != expected || r.Success != (expected == "success") || udp.Load() != 1 {
				t.Fatalf("%s: %+v", name, r)
			}
			if name == "fallback" && tcp.Load() != 1 {
				t.Fatal("truncated UDP did not retry TCP")
			}
			if !r.Success && r.Error == "" {
				t.Fatal("failure omitted detail")
			}
		})
	}
}
func TestDirectResolverCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := CheckDNSResolver(ctx, "example.test", "127.0.0.1:53", time.Second); r.Success || r.Error == "" {
		t.Fatal(r)
	}
}
