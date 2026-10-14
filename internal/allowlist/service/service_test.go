package service

import (
	"net/netip"
	"testing"
)

func TestParseNetwork(t *testing.T) {
	ok := map[string]string{
		"192.0.2.7":           "192.0.2.7/32",
		" 192.0.2.0/24 ":      "192.0.2.0/24",
		"10.0.0.0/8":          "10.0.0.0/8",
		"2001:db8::1":         "2001:db8::1/128",
		"2001:DB8::/32":       "2001:db8::/32",
		"::ffff:198.51.100.7": "198.51.100.7/32",
		"::ffff:10.1.0.0/112": "10.1.0.0/16",
		"100.64.0.0/10":       "100.64.0.0/10",
	}
	for in, want := range ok {
		p, err := ParseNetwork(in)
		if err != nil || p.String() != want {
			t.Errorf("ParseNetwork(%q) = %v, %v; want %s", in, p, err, want)
		}
	}
	for _, bad := range []string{"", "x", "300.0.0.1", "192.0.2.7/33", "10.0.0.5/24", "0.0.0.0/0", "8.0.0.0/7", "::/0",
		"2001::/15", "fe80::1%eth0", "192.0.2.0/24/1", "192.0.2.1-192.0.2.9"} {
		if _, err := ParseNetwork(bad); err == nil {
			t.Errorf("ParseNetwork(%q) accepted", bad)
		}
	}
}

func TestDecide(t *testing.T) {
	s := &Service{}
	set := func(mode string, nets ...string) {
		snap := &Snapshot{Mode: mode}
		for _, n := range nets {
			snap.Networks = append(snap.Networks, netip.MustParsePrefix(n))
		}
		s.snap.Store(snap)
	}
	in, out := netip.MustParseAddr("192.0.2.9"), netip.MustParseAddr("203.0.113.1")
	mapped := netip.MustParseAddr("::ffff:192.0.2.9")
	set(ModeOff, "192.0.2.0/24")
	if d := s.Decide(out); !d.Allow || d.Outside {
		t.Errorf("off: %+v", d)
	}
	set(ModeMonitor, "192.0.2.0/24")
	if d := s.Decide(out); !d.Allow || !d.Outside {
		t.Errorf("monitor outside: %+v", d)
	}
	if d := s.Decide(in); !d.Allow || d.Outside {
		t.Errorf("monitor inside: %+v", d)
	}
	set(ModeEnforce, "192.0.2.0/24")
	if d := s.Decide(out); d.Allow || !d.Outside {
		t.Errorf("enforce outside: %+v", d)
	}
	if d := s.Decide(mapped); !d.Allow {
		t.Errorf("enforce IPv4-mapped inside: %+v", d)
	}
	if d := s.Decide(netip.Addr{}); d.Allow {
		t.Errorf("enforce unknown IP allowed: %+v", d)
	}
	set(ModeEnforce)
	if d := s.Decide(in); d.Allow {
		t.Errorf("enforce with an empty list allowed: %+v", d)
	}
}
