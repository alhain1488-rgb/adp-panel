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

func TestStarsFor(t *testing.T) {
	// The default tariffs against the default rate of 130 kopecks per Star. Each
	// result must round UP: a Star short leaves the client unable to buy the plan
	// the button was labelled with.
	tests := []struct {
		name       string
		need, rate int64
		want       int64
	}{
		{"week 70₽", 7000, 130, 54},     // 53.85 → 54
		{"month 200₽", 20000, 130, 154}, // 153.85 → 154
		{"year 2000₽", 200000, 130, 1539},
		{"exact multiple", 13000, 130, 100},
		{"one kopeck over", 13001, 130, 101},
		{"already covered", 0, 130, 0},
		{"negative need", -500, 130, 0},
		{"unset rate", 20000, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := starsFor(tc.need, tc.rate); got != tc.want {
				t.Fatalf("starsFor(%d, %d) = %d, want %d", tc.need, tc.rate, got, tc.want)
			}
		})
	}
}

func TestStarsForCoversTheNeed(t *testing.T) {
	// The invariant the tariff buttons rely on: the quoted Stars are always worth
	// at least what the client still owes.
	const rate = 130
	for need := int64(1); need <= 5000; need++ {
		if got := starsFor(need, rate) * rate; got < need {
			t.Fatalf("starsFor(%d)*%d = %d, short of the need", need, rate, got)
		}
	}
}

func TestSignupName(t *testing.T) {
	tests := []struct {
		chatID, username, want string
	}{
		{"555001", "someone", "tg:@someone"},
		{"555002", "", "tg:555002"},
		{"555003", "   ", "tg:555003"},
	}
	for _, tc := range tests {
		if got := signupName(tc.chatID, tc.username); got != tc.want {
			t.Errorf("signupName(%q, %q) = %q, want %q", tc.chatID, tc.username, got, tc.want)
		}
	}
}
