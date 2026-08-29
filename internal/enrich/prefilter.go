package enrich

import "strings"

// Keyword pre-filter: an item goes to the LLM only if it mentions the region
// AND a threat-ish term (or comes from an inherently on-topic source). This
// cuts the bulk of general news (sports, culture, economy) before spending
// tokens. Multilingual stems cover LT/LV/ET/PL/RU where feasible.

var regionTerms = []string{
	"lithuania", "lietuv", "vilnius", "kaunas", "klaiped",
	"latvia", "latvij", "riga", "rīga",
	"estonia", "eesti", "tallinn", "narva",
	"poland", "polsk", "polish", "warsaw", "warszaw", "gdansk", "gdańsk",
	"baltic", "balti", "kaliningrad", "suwalki", "suwałki",
	"belarus", "russia", "rusij", "krievij", "venemaa", "rosja", "росси",
	"nato",
}

var threatTerms = []string{
	// kinetic / sabotage
	"sabotage", "sabotaž", "arson", "explos", "attack", "incident",
	// navigation & airspace
	"jamming", "jammed", "gps", "gnss", "spoofing", "drone", "airspace",
	"incursion", "violation",
	// maritime
	"cable", "pipeline", "shadow fleet", "tanker", "vessel",
	// cyber & info
	"cyber", "hack", "ddos", "phishing", "ransom", "disinformation",
	"propaganda", "influence operation", "fake",
	// intelligence & security
	"espionage", "spy", "intelligence", "gru", "fsb", "recruit",
	"security service", "counterintelligence",
	// military & border
	"military", "troops", "missile", "exercise", "border", "migrant",
	"mobilization", "provocation", "hybrid", "threat", "defense", "defence",
	"army", "kariuomen", "zapad",
	// energy
	"energy", "electricity", "grid", "lng",
}

// AlwaysRelevantSources skip the region/threat check — everything they
// publish is on-topic (dedicated security/disinfo outlets).
var alwaysRelevantSources = map[string]bool{
	"euvsdisinfo": true,
	"cert-pl":     true,
	"icds":        true,
}

func PassesPrefilter(source, title, body string) bool {
	if alwaysRelevantSources[source] {
		return true
	}
	text := strings.ToLower(title + " " + body)
	return containsAny(text, regionTerms) && containsAny(text, threatTerms)
}

func containsAny(text string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}
