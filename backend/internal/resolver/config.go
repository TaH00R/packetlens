package resolver

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const MaxResolvers = 3

func Normalize(value string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("resolver must be an IP:port endpoint (IPv6 in brackets)")
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("resolver requires a unicast IP and port 1..65535")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(n)), nil
}
func Parse(raw string) ([]string, error) {
	endpoints := []string{}
	if strings.TrimSpace(raw) == "" {
		return endpoints, nil
	}
	seen := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		endpoint, err := Normalize(value)
		if err != nil {
			return nil, err
		}
		if seen[endpoint] {
			return nil, fmt.Errorf("duplicate DNS resolver")
		}
		seen[endpoint] = true
		endpoints = append(endpoints, endpoint)
	}
	if len(endpoints) > MaxResolvers {
		return nil, fmt.Errorf("at most %d DNS resolvers may be configured", MaxResolvers)
	}
	return endpoints, nil
}
func ValidOutcome(outcome string, success bool) bool {
	if success {
		return outcome == "success"
	}
	switch outcome {
	case "timeout", "network_error", "dns_error", "no_answer", "invalid_response":
		return true
	}
	return false
}
