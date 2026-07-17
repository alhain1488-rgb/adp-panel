package sync

import (
	"strings"
	"testing"
)

func TestParseBlocklist(t *testing.T) {
	raw := "vk.com\n" +
		"  VK.com \n" + // duplicate (case/space) → dropped
		"# a comment\n" +
		"\n" +
		"https://facebook.com/some/path?x=1\n" + // URL → facebook.com
		"*.instagram.com\n" + // wildcard → instagram.com
		"m.example.com/\n" +
		"not a domain\n" + // no dot → dropped
		"bad_underscore.com\n" // invalid char → dropped
	got := ParseBlocklist(raw)
	want := []string{"vk.com", "facebook.com", "instagram.com", "m.example.com"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ParseBlocklist = %v, want %v", got, want)
	}
}

func TestBuildXrayConfig_Blocked(t *testing.T) {
	out, err := buildXrayConfig(nil, []string{"vk.com", "facebook.com"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"protocol": "blackhole"`, `"outboundTag": "blocked"`, `"domain:vk.com"`, `"domain:facebook.com"`, `"routing"`} {
		if !strings.Contains(s, want) {
			t.Errorf("xray config missing %q\n%s", want, s)
		}
	}
	// With no blocklist, no routing/blackhole is emitted (unchanged behaviour).
	out, _ = buildXrayConfig(nil, nil)
	if strings.Contains(string(out), "blackhole") || strings.Contains(string(out), "routing") {
		t.Errorf("empty blocklist should not add routing/blackhole:\n%s", out)
	}
}

func TestBuildSingboxConfig_Blocked(t *testing.T) {
	out, err := buildSingboxConfig(nil, []string{"vk.com"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"type": "block"`, `"outbound": "blocked"`, `"domain_suffix"`, `".vk.com"`, `"vk.com"`, `"route"`} {
		if !strings.Contains(s, want) {
			t.Errorf("sing-box config missing %q\n%s", want, s)
		}
	}
	out, _ = buildSingboxConfig(nil, nil)
	if strings.Contains(string(out), `"block"`) || strings.Contains(string(out), `"route"`) {
		t.Errorf("empty blocklist should not add route/block:\n%s", out)
	}
}
