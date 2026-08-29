package sources

import "testing"

// The dashboard ingests adversary media deliberately. Misclassifying it as
// ordinary reporting would launder a state claim into apparent fact, so this
// mapping is load-bearing.
func TestCredibilityClasses(t *testing.T) {
	cases := map[string]string{
		"tg:rybar":      CredStateControlled,
		"tg:baltnews":   CredStateControlled,
		"tg:mod_russia": CredStateControlled,
		"tass-en":       CredStateControlled,
		"ria":           CredStateControlled,
		"belta":         CredStateControlled,
		"zvezda":        CredStateControlled,
		"interfax":      CredStateControlled,

		"lrt-en":  CredInstitutional,
		"cert-lv": CredInstitutional,
		"lt-vsd":  CredInstitutional,
		"europol": CredInstitutional,
		"icds":    CredInstitutional,
		// Nordic public broadcasters sit with LRT/ERR/LSM, not with the wires.
		"yle-news":    CredInstitutional,
		"svt-nyheter": CredInstitutional,
		"becid":       CredInstitutional,

		// Exiled Russian press: independent, and must never be lumped in with
		// the state wires it exists to counter.
		"moscow-times":  CredIndependent,
		"novaya-europe": CredIndependent,
		"meduza-en":     CredIndependent,

		"tg:meduzalive":      CredIndependent, // exiled independent press
		"tg:theinsider":      CredIndependent,
		"notes-from-poland":  CredIndependent,
		"gdelt:example.com":  CredIndependent,
		"reddit:r/lithuania": CredIndependent,
	}
	for source, want := range cases {
		if got := Credibility(source); got != want {
			t.Errorf("Credibility(%q) = %q, want %q", source, got, want)
		}
	}
}

// Exiled Russian outlets report on Russia but are not controlled by it; the
// distinction is the whole point of the field.
func TestExiledPressIsNotStateControlled(t *testing.T) {
	for _, s := range []string{"tg:meduzalive", "tg:astrapress", "tg:mediazzzona", "tg:theinsider", "tg:zerkalo_io",
		"moscow-times", "novaya-europe", "meduza-en"} {
		if IsStateControlled(s) {
			t.Errorf("%s wrongly marked state-controlled", s)
		}
	}
}

func TestEveryStateSourceIsInTheExclusionList(t *testing.T) {
	list := StateControlledList()
	if len(list) != len(stateControlled) {
		t.Fatalf("exclusion list has %d entries, map has %d", len(list), len(stateControlled))
	}
	for _, s := range list {
		if Credibility(s) != CredStateControlled {
			t.Errorf("%s in exclusion list but classified %q", s, Credibility(s))
		}
	}
}
