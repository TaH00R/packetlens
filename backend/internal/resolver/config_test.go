package resolver

import "testing"

func TestEndpoints(t *testing.T) {
	got, err := Parse(" 127.0.0.1:0053, [2001:db8::1]:53 ")
	if err != nil || len(got) != 2 || got[0] != "127.0.0.1:53" || got[1] != "[2001:db8::1]:53" {
		t.Fatal(got, err)
	}
	for _, raw := range []string{"dns.example:53", "1.1.1.1", "1.1.1.1:0", "[::]:53", "224.0.0.1:53", "1.1.1.1:53,", "1.1.1.1:53,1.1.1.1:053", "1.1.1.1:53,2.2.2.2:53,3.3.3.3:53,4.4.4.4:53"} {
		if _, err := Parse(raw); err == nil {
			t.Fatal("accepted invalid resolver config", raw)
		}
	}
	if list, err := Parse(""); err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
}
