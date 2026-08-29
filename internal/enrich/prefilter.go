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
	// Cyrillic (Telegram channels post in Russian)
	"литв", "латв", "эстон", "польш", "прибалт", "калининград",
	"белорус", "беларус", "сувал", "нато",
	// Swedish and Finnish. Neither country is monitored, but the cables and
	// shadow-fleet traffic that matter to LT/LV/EE run through their waters
	// and their newsrooms report those incidents first. Without these terms a
	// Swedish-language report of a cable cut fails the region test and is
	// dropped before it ever reaches classification.
	"östersjön", "litauen", "lettland", "estland", "polen", "ryssland",
	"vitryssland", "ryska",
	"itämeri", "liettua", "latvia", "viro", "viron", "puola", "venäj",
	"valko-venäj",
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
	// equipment movements (thin captions on spotting videos must survive
	// to the LLM)
	"convoy", "column of", "echelon", "armor", "armour", "tanks", "brigade",
	"battalion", "redeploy", "reinforcement",
	// Cyrillic
	"диверс", "саботаж", "кибератак", "шпион", "разведк", "дрон", "бпла",
	"ракет", "учени", "граница", "провокаци", "гибридн", "вторжен",
	"колонн", "эшелон", "переброс", "военная техник", "войск", "танк",
	// Belarusian
	"калона", "вайск", "тэхнік",
	// Polish / Lithuanian
	"szpieg", "granic", "wojsk", "zagro", "czołg", "kolumna", "przerzut",
	"šnip", "kibernet", "pasien", "grėsm", "kolona", "kariuomen",
	// Swedish ("sabotage", "attack", "drönare" share roots with English but
	// "kabel" and "störning" do not, and cable damage is the whole reason
	// these feeds are watched)
	"kabel", "störning", "spionage", "intrång", "ubåt", "fartyg", "militär",
	"gräns", "hot", "försvar", "ledning",
	// Finnish
	"kaapeli", "häirin", "vakoilu", "lennokki", "droni", "alus", "sotilas",
	"raja", "uhka", "puolustus",
}

// AlwaysRelevantSources skip the region/threat check — everything they
// publish is on-topic (dedicated security/disinfo outlets).
var alwaysRelevantSources = map[string]bool{
	"euvsdisinfo": true,
	"cert-pl":     true,
	"icds":        true,
}

// regionImpliedSources cover a monitored country by definition, so their
// posts only need a threat term — spotting captions name towns ("колонна в
// Гродно"), not countries.
var regionImpliedSources = map[string]bool{
	"tg:MotolkoHelp": true, // Belarus monitoring
	"tg:belzhd_live": true, // Belarusian railways
	"tg:nexta_live":  true, // Belarus
}

func PassesPrefilter(source, title, body string) bool {
	if alwaysRelevantSources[source] {
		return true
	}
	text := strings.ToLower(title + " " + body)
	// A region-scoped source (reddit:r/lithuania, tg channel about Belarus)
	// implies the region; posts there rarely restate the country name.
	regionOK := regionImpliedSources[source] ||
		containsAny(text, regionTerms) ||
		containsAny(strings.ToLower(source), regionTerms)
	return regionOK && containsAny(text, threatTerms)
}

func containsAny(text string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}
