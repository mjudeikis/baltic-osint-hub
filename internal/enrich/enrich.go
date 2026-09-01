// Package enrich classifies raw feed items into threat incidents using the
// OpenAI API. Items are sent in batches; the model returns one JSON verdict
// per item.
package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Categories is the classification taxonomy. Kept in sync with the frontend.
var Categories = []string{
	"sabotage", "undersea-infrastructure", "gps-jamming", "airspace-border",
	"cyber", "disinformation", "espionage", "military", "energy", "political",
}

// Countries uses ISO 3166-1 alpha-2, limited to the monitored region.
var Countries = []string{"LT", "LV", "EE", "PL"}

// Tone is the direction of an item for regional security — the dashboard
// would otherwise read as uniformly dire, when much of the reporting is
// defensive progress rather than adversary success.
const (
	ToneNegative = "negative" // adversary succeeded, or the region lost ground
	TonePositive = "positive" // the region gained ground: defence, interdiction, resilience
	ToneNeutral  = "neutral"  // analysis, statements, routine activity, context
)

var Tones = []string{TonePositive, ToneNeutral, ToneNegative}

type Verdict struct {
	ID        int64    `json:"id"`
	Relevant  bool     `json:"relevant"`
	Category  string   `json:"category"`
	Countries []string `json:"countries"`
	Severity  int      `json:"severity"`
	Tone      string   `json:"tone"`
	Place     string   `json:"place"`
	Summary   string   `json:"summary"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
}

type Classifier struct {
	client *openAIClient
	model  string
}

// NewClassifier builds an OpenAI-backed classifier. baseURL is normally empty
// (api.openai.com) and exists for tests and compatible gateways.
func NewClassifier(apiKey, model, baseURL string) *Classifier {
	return &Classifier{
		client: newOpenAIClient(apiKey, baseURL),
		model:  model,
	}
}

const systemPrompt = `You classify news items for a public dashboard tracking Russian and Belarusian hybrid threats against Lithuania (LT), Latvia (LV), Estonia (EE), and Poland (PL).

For each numbered item, decide whether it reports a concrete threat-related event or development affecting those countries. Relevant categories:
- sabotage: arson, vandalism, rail/infrastructure sabotage on land
- undersea-infrastructure: damage or threats to sea cables, pipelines; shadow-fleet activity
- gps-jamming: GNSS jamming or spoofing
- airspace-border: airspace violations, drones, border incidents, instrumentalized migration
- cyber: cyberattacks, DDoS, ransomware, state-linked intrusions
- disinformation: influence operations, propaganda campaigns, information manipulation
- espionage: spying, agent recruitment, espionage arrests or expulsions
- military: Russian/Belarusian military activity, exercises (e.g. Zapad), posture changes near the region; NATO responses
- energy: threats to power grids, energy supply, LNG terminals
- political: explicit threats, ultimatums, or coercive diplomacy directed at these countries

NOT relevant: general politics, economy, sports, culture, Ukraine-war battlefield news without direct Baltic/Poland connection, EU policy without a threat dimension, historical retrospectives.

Severity scale (how consequential, regardless of who benefits): 1 = analysis/statements, 2 = minor incident or elevated rhetoric, 3 = notable incident (jamming episode, arrest, small sabotage), 4 = serious incident (infrastructure damaged, airspace violated by military aircraft, major cyberattack), 5 = critical (casualties, article-5-adjacent, major infrastructure destroyed).

Severity 4 and 5 require that SOMETHING HAPPENED. They describe an event, not a condition and not a warning. Reporting about capability, readiness, munitions or equipment stocks, force levels, budgets, procurement, vulnerability assessments, war-gaming, or what could happen in a future conflict is severity 1 or 2 no matter how grave the wording — "critical shortage", "not enough to stop an attack" and "gravest threat since the Cold War" are all still analysis. If you cannot name a discrete thing that occurred on a date, the item is not a 4.

Tone — judge every item from the perspective of the security of Lithuania, Latvia, Estonia and Poland, and be honest in both directions:
- "negative": the adversary succeeded or the region lost ground. Sabotage that worked, airspace actually violated, a cyberattack that landed, a new threat capability deployed against the region, coercion that achieved its aim.
- "positive": the region gained ground. Saboteurs or spies arrested, charged or convicted; an attack thwarted or intercepted; air defence, troops or capabilities deployed or delivered to the region; allied reinforcement; new funding or infrastructure for defence; sanctions or expulsions imposed; resilience improving; de-escalation.
- "neutral": analysis, commentary, statements without action, routine exercises, historical or background reporting, items where the direction is genuinely unclear.

Tone and severity are independent. A large NATO reinforcement is severity 3-4 and "positive". A successful arson attack is severity 4 and "negative". Do not mark something "negative" merely because it concerns a threat — an article about catching a saboteur is "positive" even though the subject is sabotage. Most defence-procurement and exercise news is "positive" or "neutral", not "negative".

Countries — list ONLY where the event itself took place or whose territory, institutions, infrastructure or citizens were directly involved. Not countries that are merely nearby, alarmed, implicated by analogy, or part of the same region. Drones entering Estonian airspace is ["EE"] even if neighbours scrambled jets or commented — the dashboard maps each country's own incidents, and tagging neighbours marks a Lithuanian reader's country with an Estonian event. List several countries only when the event genuinely spans them: a cable linking two of them, coordinated arrests in three, an exercise on the borders of all four. Regional analysis naming no specific country gets the countries it is actually about, not all four by default.

Items may be in Lithuanian, Latvian, Estonian, Polish, Russian, or English. Always write the summary in English, one to two factual sentences.

Location: give lat/lon whenever the item names ANY place you can place on a map — a city or town, a district, an airport or port, a military base, a border crossing, a named stretch of sea, or a region. Use the centre of that place; approximate is fine and expected. Also give "place" as the name you geocoded, so a reader can tell what the pin refers to.
- Named settlement or facility (Vilnius, Narva, Baltiysk, Šiauliai air base) → its coordinates.
- A region or district (Utena county, Kaliningrad oblast) → the centre of that region.
- A sea area (Gulf of Finland, Baltic Sea near Gotland) → a representative point in it.
- Only a country and nothing more specific → null. Do NOT fall back to the capital, because that would put a false pin on a real city.
- No place at all (general analysis, policy commentary) → null.
Most items that describe a real event do name a place; use it rather than defaulting to null.

Reply with ONLY a JSON array, one object per item:
[{"id": <item id>, "relevant": true|false, "category": "<one of the categories>", "countries": ["LT"|"LV"|"EE"|"PL", ...], "severity": 1-5, "tone": "positive"|"negative"|"neutral", "summary": "<english summary>", "place": "<name you geocoded, or null>", "lat": null|number, "lon": null|number}]
For irrelevant items only id and relevant=false are needed.`

// ClassifyBatch sends up to ~20 items and returns verdicts keyed by item ID.
func (c *Classifier) ClassifyBatch(ctx context.Context, items []store.RawItem) (map[int64]Verdict, error) {
	var sb strings.Builder
	for _, it := range items {
		fmt.Fprintf(&sb, "### Item %d\nSource: %s\nTitle: %s\n", it.ID, it.Source, it.Title)
		if it.Body != "" {
			fmt.Fprintf(&sb, "Text: %s\n", it.Body)
		}
		sb.WriteString("\n")
	}

	text, err := c.client.complete(ctx, c.model, systemPrompt, sb.String(), 16000)
	if err != nil {
		return nil, err
	}
	verdicts, err := parseVerdicts(text)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w (raw: %.200s)", err, text)
	}
	out := make(map[int64]Verdict, len(verdicts))
	for _, v := range verdicts {
		out[v.ID] = v
	}
	return out, nil
}

// parseVerdicts extracts the JSON array from the model reply, tolerating
// code fences or surrounding prose.
func parseVerdicts(text string) ([]Verdict, error) {
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found")
	}
	var verdicts []Verdict
	if err := json.Unmarshal([]byte(text[start:end+1]), &verdicts); err != nil {
		return nil, err
	}
	valid := map[string]bool{}
	for _, c := range Categories {
		valid[c] = true
	}
	validTone := map[string]bool{}
	for _, t := range Tones {
		validTone[t] = true
	}
	for i := range verdicts {
		v := &verdicts[i]
		if !v.Relevant {
			continue
		}
		if !valid[v.Category] {
			v.Category = "political"
		}
		// Unknown or missing tone must not silently become "negative" —
		// neutral is the honest default when the model didn't commit.
		if !validTone[v.Tone] {
			v.Tone = ToneNeutral
		}
		if v.Severity < 1 {
			v.Severity = 1
		}
		if v.Severity > 5 {
			v.Severity = 5
		}
		if len(v.Countries) == 0 {
			v.Relevant = false
		}
	}
	return verdicts, nil
}
