package target

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func config(address string) Config {
	return Config{Type: "tcp", Address: address, Enabled: true, IntervalMs: 5000, TimeoutMs: 3000}
}
func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{{" EXAMPLE.COM.:00443 ", "example.com:443"}, {"[2001:db8::1]:443", "[2001:db8::1]:443"}, {"127.0.0.1:80", "127.0.0.1:80"}} {
		c, err := Normalize(config(tc.input))
		if err != nil || c.Address != tc.want {
			t.Fatal(c, err)
		}
	}
	for _, address := range []string{"https://example.com:443", "a:0", "a:65536", "a:abc", "a:-1", "a:80/path", "[::]:80", "224.0.0.1:80", "a_b:80", "-a:80", "a..b:80", "[fe80::1%eth0]:80"} {
		if _, err := Normalize(config(address)); err == nil {
			t.Fatalf("accepted %s", address)
		}
	}
	for _, address := range []string{"https://example.com", "127.0.0.1", "*.example.com", "example.com:53", "a..b", "éxample.com"} {
		c := config(address)
		c.Type = "dns"
		if _, err := Normalize(c); err == nil {
			t.Fatal("accepted DNS", address)
		}
	}
	c := config(" Example.COM. ")
	c.Type = "dns"
	if n, err := Normalize(c); err != nil || n.Address != "example.com" {
		t.Fatal(n, err)
	}
	for _, change := range []func(*Config){func(c *Config) { c.IntervalMs = 4999 }, func(c *Config) { c.IntervalMs = 3600001 }, func(c *Config) { c.TimeoutMs = 99 }, func(c *Config) { c.TimeoutMs = 5001 }, func(c *Config) { c.Type = "icmp" }, func(c *Config) { c.ID = "bad" }} {
		c := config("a:80")
		change(&c)
		if _, err := Normalize(c); err == nil {
			t.Fatal("invalid config accepted", c)
		}
	}
}

type fakeRepo struct {
	configs []Config
	fail    bool
}

func (r *fakeRepo) ListTargets(context.Context) ([]Config, error) {
	return append([]Config{}, r.configs...), nil
}
func (r *fakeRepo) ReplaceTargets(ctx context.Context, configs []Config) error {
	if r.fail {
		return errors.New("injected database error")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.configs = append([]Config{}, configs...)
	return nil
}
func TestManagerAtomicityLimitsAndConcurrentEdits(t *testing.T) {
	r := &fakeRepo{}
	var applied []Config
	m, err := NewManager(context.Background(), r, func(c []Config) { applied = c })
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.Save(context.Background(), config("a:80"), true)
	if err != nil || !ValidID(a.ID) || len(applied) != 1 {
		t.Fatal(a, err)
	}
	if _, err = m.Save(context.Background(), config("A:080"), true); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate accepted", err)
	}
	r.fail = true
	a.Enabled = false
	if _, err = m.Save(context.Background(), a, false); err == nil || !m.List()[0].Enabled || !applied[0].Enabled {
		t.Fatal("failed save changed live config")
	}
	r.fail = false
	if _, err = m.Save(context.Background(), a, false); err != nil || m.List()[0].Enabled {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 1; i < MaxTargets; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := m.Save(context.Background(), config(fmt.Sprintf("host%d:443", i)), true); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if _, err = m.Save(context.Background(), config("overflow:443"), true); !errors.Is(err, ErrConflict) {
		t.Fatal("limit not enforced", err)
	}
	copy := m.List()
	copy[0].Address = "mutated"
	if m.List()[0].Address == "mutated" {
		t.Fatal("mutable list leaked")
	}
	if err = m.Delete(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(context.Background(), a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = m.Save(ctx, config("cancelled:443"), true); err == nil {
		t.Fatal("canceled save applied")
	}
}
