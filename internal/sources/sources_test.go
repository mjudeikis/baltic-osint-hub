package sources

import "testing"

func TestContentHashNormalizes(t *testing.T) {
	a := ContentHash("Russia  Jams GPS over Baltic")
	b := ContentHash("russia jams gps over baltic")
	if a != b {
		t.Error("hash should ignore case and whitespace")
	}
	if a == ContentHash("different title") {
		t.Error("different titles must not collide")
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML(`<p>Drone&nbsp;spotted <a href="#">near   border</a></p>`)
	want := "Drone spotted near border"
	if got != want {
		t.Errorf("stripHTML = %q, want %q", got, want)
	}
}

// Links are rendered as anchors on a public page; only http(s) may get in.
func TestValidLink(t *testing.T) {
	cases := map[string]bool{
		"https://example.test/a":       true,
		"http://example.test/a?x=1":    true,
		"HTTPS://example.test":         true,
		"javascript:alert(1)":          false,
		"data:text/html;base64,PHNjcg": false,
		"ftp://example.test/f":         false,
		"file:///etc/passwd":           false,
		"/relative/path":               false,
		"":                             false,
		"not a url":                    false,
	}
	for in, want := range cases {
		if got := ValidLink(in); got != want {
			t.Errorf("ValidLink(%q) = %v, want %v", in, got, want)
		}
	}
}
