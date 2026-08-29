package sources

import "strings"

// Credibility classes. The dashboard deliberately ingests Russian and
// Belarusian state media so the narrative aimed at the region is visible —
// but presenting those claims alongside national broadcasters without
// distinction would turn a monitoring tool into an amplifier. Every item
// carries its class so the UI can mark it and the posture reading can exclude
// it.
const (
	// CredInstitutional: governments, CERTs, EU agencies, defence ministries,
	// national public broadcasters, established research institutes.
	CredInstitutional = "institutional"
	// CredIndependent: independent journalism, including exiled Russian and
	// Belarusian outlets, plus aggregators and open community sources.
	CredIndependent = "independent"
	// CredStateControlled: outlets owned by, or operating under the editorial
	// control of, the Russian or Belarusian state. Monitored as evidence of
	// messaging, never treated as reporting.
	CredStateControlled = "state-controlled"
)

// stateControlled lists sources whose output is adversary messaging.
var stateControlled = map[string]bool{
	"tg:rybar":       true, // milblogger, closely aligned with MoD lines
	"tg:mod_russia":  true, // Russian MoD official channel
	"tg:milinfolive": true,
	"tg:baltnews":    true, // Kremlin outlet targeting Baltic audiences
	"tass-en":        true,
	"tass-ru":        true,
	"ria":            true,
	"zvezda":         true, // Russian MoD broadcaster
	"belta":          true, // Belarusian state agency
	// Interfax and Kommersant are nominally private but operate under Russian
	// media law and wartime censorship. Marked conservatively: over-marking
	// costs a label, under-marking launders a claim.
	"interfax":   true,
	"kommersant": true,
}

// institutional lists official and public-service sources.
var institutional = map[string]bool{
	"lrt-en": true, "err-news": true, "err-et": true, "err-ru": true,
	"lsm-en": true, "lsm-lv": true,
	"cert-pl": true, "cert-lv": true, "ee-ria": true, "lt-vsd": true,
	"lt-mod": true, "ee-mil": true, "ee-mod": true, "lv-mod": true,
	"lv-mod-lv": true, "ee-gov": true, "ee-mfa": true,
	"frontex": true, "europol": true, "ec-press": true, "euvsdisinfo": true,
	"elering": true,
	// Nordic public broadcasters, on the same footing as LRT/ERR/LSM.
	"yle-news": true, "svt-nyheter": true,
	// EDMO regional fact-checking hubs (Univ. of Tartu and partners).
	"becid": true, "fact-hub": true,
}

// Credibility classifies a source name as recorded on raw_items.
func Credibility(source string) string {
	if stateControlled[source] {
		return CredStateControlled
	}
	if institutional[source] {
		return CredInstitutional
	}
	// Research institutes and think tanks read as institutional analysis.
	switch source {
	case "cepa", "jamestown", "icds", "warsaw-institute", "osw-warsaw", "disinfolab":
		return CredInstitutional
	}
	// GDELT re-publishes whatever it indexed; the underlying domain is in the
	// source name but its editorial character is unknown to us.
	if strings.HasPrefix(source, "gdelt:") {
		return CredIndependent
	}
	return CredIndependent
}

// IsStateControlled is the check the posture calculation uses.
func IsStateControlled(source string) bool { return stateControlled[source] }

// StateControlledList returns the state-controlled source names, for the
// store's posture exclusion.
func StateControlledList() []string {
	out := make([]string, 0, len(stateControlled))
	for name := range stateControlled {
		out = append(out, name)
	}
	return out
}
