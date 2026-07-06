package brand

import (
	"strings"
	"testing"
)

func TestTextFooter_Bilingual(t *testing.T) {
	f := TextFooter()
	if !strings.Contains(f, NameRU) || !strings.Contains(f, "("+NameEN+")") {
		t.Fatalf("footer not bilingual: %q", f)
	}
}

func TestHTMLFooter_WithAndWithoutImage(t *testing.T) {
	withImg := HTMLFooter("https://p.example/cheremsha.png")
	if !strings.Contains(withImg, `src="https://p.example/cheremsha.png"`) {
		t.Fatalf("image not referenced: %q", withImg)
	}
	if !strings.Contains(withImg, NameRU) || !strings.Contains(withImg, NameEN) {
		t.Fatal("html footer not bilingual")
	}
	noImg := HTMLFooter("")
	if strings.Contains(noImg, "<img") {
		t.Fatalf("expected no <img> when url empty: %q", noImg)
	}
	if !strings.Contains(noImg, Emoji) {
		t.Fatal("emoji fallback missing")
	}
}
