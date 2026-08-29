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
