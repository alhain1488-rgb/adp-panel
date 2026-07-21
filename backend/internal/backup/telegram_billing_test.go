package backup

import "testing"

func TestParseTopupPayload(t *testing.T) {
	cases := []struct {
		in         string
		wantClient int64
		wantStars  int64
		wantOK     bool
	}{
		{"topup:42:100", 42, 100, true},
		{"topup:1:1", 1, 1, true},
		{"topup:42:0", 0, 0, false},   // zero stars rejected
		{"topup:0:100", 0, 0, false},  // zero client rejected
		{"topup:42", 0, 0, false},     // too few parts
		{"buy:42:100", 0, 0, false},   // wrong prefix
		{"topup:x:100", 0, 0, false},  // non-numeric client
		{"topup:42:abc", 0, 0, false}, // non-numeric stars
		{"", 0, 0, false},
	}
	for _, c := range cases {
		cid, stars, ok := parseTopupPayload(c.in)
		if ok != c.wantOK || cid != c.wantClient || stars != c.wantStars {
			t.Errorf("parseTopupPayload(%q) = (%d,%d,%v), want (%d,%d,%v)",
				c.in, cid, stars, ok, c.wantClient, c.wantStars, c.wantOK)
		}
	}
}

func TestFormatRubles(t *testing.T) {
	cases := map[int64]string{
		0:      "0 ₽",
		20000:  "200 ₽",
		7000:   "70 ₽",
		7050:   "70,50 ₽",
		130:    "1,30 ₽",
		-20000: "−200 ₽",
		200000: "2000 ₽",
	}
	for k, want := range cases {
		if got := formatRubles(k); got != want {
			t.Errorf("formatRubles(%d) = %q, want %q", k, got, want)
		}
	}
	if got := formatRublesSigned(20000); got != "+200 ₽" {
		t.Errorf("formatRublesSigned(20000) = %q, want +200 ₽", got)
	}
	if got := formatRublesSigned(-20000); got != "−200 ₽" {
		t.Errorf("formatRublesSigned(-20000) = %q, want −200 ₽", got)
	}
}

func TestBillingCommand(t *testing.T) {
	cases := map[string]string{
		statusButtonLabel:  "status",
		topupButtonLabel:   "topup",
		buyButtonLabel:     "buy",
		historyButtonLabel: "history",
		supportButtonLabel: "support",
		"/status":          "status",
		"/balance@mybot":   "status",
		"/buy 123":         "buy",
		"/topup":           "topup",
		"/history":         "history",
		"/support":         "support",
		"hello":            "",
		"/start abc":       "",
	}
	for in, want := range cases {
		if got := billingCommand(in); got != want {
			t.Errorf("billingCommand(%q) = %q, want %q", in, got, want)
		}
	}
}
