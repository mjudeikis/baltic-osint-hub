// Package enrich classifies raw feed items into threat incidents using the
// Claude API. Items are sent in batches; the model returns one JSON verdict
// per item.
package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Categories is the classification taxonomy. Kept in sync with the frontend.
var Categories = []string{
	"sabotage", "undersea-infrastructure", "gps-jamming", "airspace-border",
	"cyber", "disinformation", "espionage", "military", "energy", "political",
}

// Countries uses ISO 3166-1 alpha-2, limited to the monitored region.
var Countries = []string{"LT", "LV", "EE", "PL"}

type Verdict struct {
	ID        int64    `json:"id"`
	Relevant  bool     `json:"relevant"`
	Category  string   `json:"category"`
	Countries []string `json:"countries"`
	Severity  int      `json:"severity"`
	Summary   string   `json:"summary"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
}

type Classifier struct {
	client anthropic.Client
	model  string
}

func NewClassifier(apiKey, model string) *Classifier {
	return &Classifier{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
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

Severity scale: 1 = analysis/statements, 2 = minor incident or elevated rhetoric, 3 = notable incident (jamming episode, arrest, small sabotage), 4 = serious incident (infrastructure damaged, airspace violated by military aircraft, major cyberattack), 5 = critical (casualties, article-5-adjacent, major infrastructure destroyed).

Items may be in Lithuanian, Latvian, Estonian, Polish, Russian, or English. Always write the summary in English, one to two factual sentences. If the text names a specific location, give approximate lat/lon; otherwise null.

Reply with ONLY a JSON array, one object per item:
[{"id": <item id>, "relevant": true|false, "category": "<one of the categories>", "countries": ["LT"|"LV"|"EE"|"PL", ...], "severity": 1-5, "summary": "<english summary>", "lat": null|number, "lon": null|number}]
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

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 8000,
		System: []anthropic.TextBlockParam{{
			Text:         systemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += tb.Text
		}
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
	for i := range verdicts {
		v := &verdicts[i]
		if !v.Relevant {
			continue
		}
		if !valid[v.Category] {
			v.Category = "political"
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
