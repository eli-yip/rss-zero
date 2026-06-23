package tombkeeper

import "testing"

// realPairs are genuine (mid, bid) pairs extracted from tombkeeper.io flight
// data; see example/README.md §6. They are the decisive correctness test.
var realPairs = []struct{ mid, bid string }{
	{"5310058709647658", "R4dFIEtNo"},
	{"5312665532239202", "R5juh9owa"},
	{"5310369857801007", "R4lLzwJoH"},
	{"5307458244575582", "R381qjcjI"},
	{"5310492706083175", "R4oXIpwvJ"},
	{"5312623991326758", "R5iph5z9k"},
	{"5310878589392289", "R4z06Dpmx"},
}

func TestMidBidRoundTrip(t *testing.T) {
	for _, p := range realPairs {
		if got := MidToBid(p.mid); got != p.bid {
			t.Errorf("MidToBid(%s) = %s, want %s", p.mid, got, p.bid)
		}
		got, err := BidToMid(p.bid)
		if err != nil {
			t.Errorf("BidToMid(%s) error: %v", p.bid, err)
			continue
		}
		if got != p.mid {
			t.Errorf("BidToMid(%s) = %s, want %s", p.bid, got, p.mid)
		}
	}
}

func TestBidToMidInvalid(t *testing.T) {
	if _, err := BidToMid("R5ju!9owa"); err == nil {
		t.Error("expected error for invalid base62 char, got nil")
	}
	if _, err := BidToMid(""); err == nil {
		t.Error("expected error for empty bid, got nil")
	}
	// A non-leftmost 4-char group can decode to 8 digits (>= 10^7), which is not a
	// real 7-digit mid block; it must return an error rather than panic on a
	// negative left-pad count ("ZZZZ" = 14776335).
	if _, err := BidToMid("AZZZZ"); err == nil {
		t.Error("expected error for an out-of-range bid group, got nil")
	}
}

func TestMidBidShort(t *testing.T) {
	// Short mids produce a single unpadded base62 chunk and still round-trip.
	for _, mid := range []string{"1", "200", "9999999"} {
		bid := MidToBid(mid)
		got, err := BidToMid(bid)
		if err != nil {
			t.Fatalf("BidToMid(%s) error: %v", bid, err)
		}
		if got != mid {
			t.Errorf("round trip %s -> %s -> %s", mid, bid, got)
		}
	}
}
