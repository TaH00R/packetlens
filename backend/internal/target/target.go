package target

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

const MaxTargets = 32

var ErrInvalid = errors.New("invalid target configuration")
var ErrNotFound = errors.New("target not found")
var ErrConflict = errors.New("target already exists or target limit reached")

type Config struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Address    string `json:"address"`
	Enabled    bool   `json:"enabled"`
	IntervalMs int    `json:"interval_ms"`
	TimeoutMs  int    `json:"timeout_ms"`
}

func ValidID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && hex.EncodeToString(b) == id
}
func hostname(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
func Normalize(c Config) (Config, error) {
	bad := func(message string) (Config, error) { return c, fmt.Errorf("%w: %s", ErrInvalid, message) }
	if c.ID != "" && !ValidID(c.ID) {
		return bad("invalid ID")
	}
	c.Address = strings.ToLower(strings.TrimSpace(c.Address))
	switch c.Type {
	case "tcp":
		host, port, err := net.SplitHostPort(c.Address)
		if err != nil {
			return bad("TCP address must be host:port (IPv6 in brackets)")
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return bad("port must be 1..65535")
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsUnspecified() || ip.IsMulticast() {
				return bad("unspecified and multicast addresses are not supported")
			}
			host = ip.String()
		} else {
			host = strings.TrimSuffix(host, ".")
			if !hostname(host) {
				return bad("invalid TCP hostname")
			}
		}
		c.Address = net.JoinHostPort(host, strconv.Itoa(n))
	case "dns":
		c.Address = strings.TrimSuffix(c.Address, ".")
		if !hostname(c.Address) || net.ParseIP(c.Address) != nil {
			return bad("DNS target must be an ASCII domain name, not an IP or URL")
		}
	default:
		return bad("type must be tcp or dns")
	}
	if c.IntervalMs < 5000 || c.IntervalMs > 3600000 {
		return bad("interval must be 5000..3600000 milliseconds")
	}
	if c.TimeoutMs < 100 || c.TimeoutMs > 30000 || c.TimeoutMs > c.IntervalMs {
		return bad("timeout must be 100..30000 milliseconds and no greater than interval")
	}
	return c, nil
}
func ValidateSet(configs []Config) error {
	if len(configs) > MaxTargets {
		return ErrConflict
	}
	ids := map[string]bool{}
	addresses := map[string]bool{}
	for _, c := range configs {
		n, err := Normalize(c)
		if err != nil {
			return err
		}
		if !ValidID(c.ID) || n != c {
			return fmt.Errorf("%w: configuration is not canonical", ErrInvalid)
		}
		key := c.Type + ":" + c.Address
		if ids[c.ID] || addresses[key] {
			return ErrConflict
		}
		ids[c.ID] = true
		addresses[key] = true
	}
	return nil
}

// Repository replaces a bounded configuration set atomically. Only Manager writes
// it at runtime; measurements and detector history are stored independently.
type Repository interface {
	ListTargets(context.Context) ([]Config, error)
	ReplaceTargets(context.Context, []Config) error
}
type Manager struct {
	mu      sync.Mutex
	repo    Repository
	apply   func([]Config)
	configs []Config
}

func NewManager(ctx context.Context, repo Repository, apply func([]Config)) (*Manager, error) {
	configs, err := repo.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	if err := ValidateSet(configs); err != nil {
		return nil, err
	}
	m := &Manager{repo: repo, apply: apply, configs: configs}
	m.apply(append([]Config{}, configs...))
	return m, nil
}
func (m *Manager) List() []Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Config{}, m.configs...)
}
func (m *Manager) Save(ctx context.Context, c Config, create bool) (Config, error) {
	c, err := Normalize(c)
	if err != nil {
		return c, err
	}
	if create {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return c, err
		}
		c.ID = hex.EncodeToString(b[:])
	} else if !ValidID(c.ID) {
		return c, ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := append([]Config{}, m.configs...)
	found := false
	for i, old := range next {
		if old.ID == c.ID {
			next[i] = c
			found = true
			break
		}
	}
	if create {
		next = append(next, c)
	} else if !found {
		return c, ErrNotFound
	}
	if err := m.commit(ctx, next); err != nil {
		return c, err
	}
	return c, nil
}
func (m *Manager) Delete(ctx context.Context, id string) error {
	if !ValidID(id) {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := []Config{}
	found := false
	for _, c := range m.configs {
		if c.ID == id {
			found = true
		} else {
			next = append(next, c)
		}
	}
	if !found {
		return ErrNotFound
	}
	return m.commit(ctx, next)
}
func (m *Manager) commit(ctx context.Context, next []Config) error {
	if err := ValidateSet(next); err != nil {
		return err
	}
	if err := m.repo.ReplaceTargets(ctx, next); err != nil {
		return fmt.Errorf("persist targets: %w", err)
	}
	m.configs = next
	m.apply(append([]Config{}, next...))
	return nil
}
