package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/resolver"
	"golang.org/x/net/dns/dnsmessage"
)

type ResolverResult struct {
	DNSResult
	Outcome string
}

// CheckDNSResolver sends a recursive A query directly to an explicit resolver.
// It bypasses OS hosts/search rules, retries truncated UDP replies over TCP, and
// uses one deadline across both exchanges. It does not perform DNSSEC validation.
func CheckDNSResolver(parent context.Context, domain, endpoint string, timeout time.Duration) (result ResolverResult) {
	result.Domain = domain
	result.Outcome = "network_error"
	start := time.Now()
	defer func() { result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000 }()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	address, err := resolver.Normalize(endpoint)
	if err != nil {
		result.Error = "Invalid resolver endpoint"
		return
	}
	name, err := dnsmessage.NewName(strings.TrimSuffix(domain, ".") + ".")
	if err != nil {
		result.Error = "Invalid DNS name"
		return
	}
	var id [2]byte
	if _, err = rand.Read(id[:]); err != nil {
		result.Error = "Could not generate DNS query ID"
		return
	}
	question := dnsmessage.Question{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}
	query := dnsmessage.Message{Header: dnsmessage.Header{ID: binary.BigEndian.Uint16(id[:]), RecursionDesired: true}, Questions: []dnsmessage.Question{question}}
	wire, err := query.Pack()
	if err != nil {
		result.Error = "Could not encode DNS query"
		return
	}
	for _, network := range []string{"udp", "tcp"} {
		packet, err := dnsExchange(ctx, network, address, wire)
		if err != nil {
			var timeoutError net.Error
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &timeoutError) && timeoutError.Timeout()) {
				result.Outcome = "timeout"
				result.Error = "DNS query timed out"
			} else {
				result.Error = fmt.Sprintf("DNS transport failed: %v", err)
			}
			return
		}
		var parser dnsmessage.Parser
		header, err := parser.Start(packet)
		if err != nil || !header.Response || header.ID != query.ID || header.OpCode != 0 {
			result.Outcome = "invalid_response"
			result.Error = "Invalid DNS response header"
			return
		}
		if header.Truncated {
			if network == "udp" {
				continue
			}
			result.Outcome = "invalid_response"
			result.Error = "Truncated DNS TCP response"
			return
		}
		questions, err := parser.AllQuestions()
		if err != nil || len(questions) != 1 || !strings.EqualFold(questions[0].Name.String(), question.Name.String()) || questions[0].Type != question.Type || questions[0].Class != question.Class {
			result.Outcome = "invalid_response"
			result.Error = "DNS response question did not match"
			return
		}
		if header.RCode != dnsmessage.RCodeSuccess {
			result.Outcome = "dns_error"
			result.Error = fmt.Sprintf("DNS response: %s", header.RCode.String())
			return
		}
		var response dnsmessage.Message
		if err := response.Unpack(packet); err != nil {
			result.Outcome = "invalid_response"
			result.Error = "Malformed DNS response"
			return
		}
		// Accept A records only for the requested name or its answer CNAME chain.
		allowed := map[string]bool{strings.ToLower(name.String()): true}
		for i := 0; i < 16; i++ {
			changed := false
			for _, answer := range response.Answers {
				if cname, ok := answer.Body.(*dnsmessage.CNAMEResource); ok && answer.Header.Class == dnsmessage.ClassINET && allowed[strings.ToLower(answer.Header.Name.String())] && !allowed[strings.ToLower(cname.CNAME.String())] {
					allowed[strings.ToLower(cname.CNAME.String())] = true
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		for _, answer := range response.Answers {
			if a, ok := answer.Body.(*dnsmessage.AResource); ok && answer.Header.Class == dnsmessage.ClassINET && allowed[strings.ToLower(answer.Header.Name.String())] {
				result.Addresses = append(result.Addresses, net.IP(a.A[:]).String())
			}
		}
		if len(result.Addresses) == 0 {
			result.Outcome = "no_answer"
			result.Error = "No A records in DNS answer"
			return
		}
		result.Success = true
		result.Outcome = "success"
		return
	}
	return
}
func dnsExchange(ctx context.Context, network, address string, query []byte) ([]byte, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if network == "udp" {
		if _, err = conn.Write(query); err != nil {
			return nil, err
		}
		packet := make([]byte, 512)
		n, err := conn.Read(packet)
		return packet[:n], err
	}
	packet := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(packet, uint16(len(query)))
	copy(packet[2:], query)
	for len(packet) > 0 {
		n, err := conn.Write(packet)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, io.ErrNoProgress
		}
		packet = packet[n:]
	}
	var prefix [2]byte
	if _, err = io.ReadFull(conn, prefix[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint16(prefix[:]))
	if size < 12 {
		return nil, fmt.Errorf("short DNS response")
	}
	packet = make([]byte, size)
	_, err = io.ReadFull(conn, packet)
	return packet, err
}
