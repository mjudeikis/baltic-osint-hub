package layers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// OpenSky snapshots aircraft in the border sectors and records notable
// sightings. Works anonymously (tight rate limits) or with OAuth2 client
// credentials from an OpenSky account (recommended).
type OpenSky struct {
	ClientID     string
	ClientSecret string
	Client       *http.Client
}

// Callsign prefixes worth flagging: Russian Air Force (RFF), the Rossiya
// special flight detachment (RSD, government aircraft), and NATO mission
// callsigns (E-3A AWACS and allied support flights fly as NATOxx).
var watchlistCallsigns = []string{"RFF", "RSD", "NATO"}

const airDedupeWindow = 6 * time.Hour

func (o *OpenSky) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	token := ""
	if o.ClientID != "" && o.ClientSecret != "" {
		t, err := o.fetchToken(ctx)
		if err != nil {
			return fmt.Errorf("opensky auth: %w", err)
		}
		token = t
	}
	sightings := 0
	for _, box := range BorderSectors {
		states, err := o.states(ctx, token, box)
		if err != nil {
			// One failing box (rate limit) shouldn't kill the others.
			log.Warn("opensky box failed", "box", box.Name, "err", err)
			continue
		}
		for _, st := range states {
			if reason := notable(st); reason != "" {
				added, err := db.InsertAirSighting(ctx, &store.AirSighting{
					ICAO24:   st.icao24,
					Callsign: st.callsign,
					Country:  st.country,
					Box:      box.Name,
					Lat:      st.lat,
					Lon:      st.lon,
					Altitude: st.altitude,
					Velocity: st.velocity,
					Reason:   reason,
				}, airDedupeWindow)
				if err != nil {
					return err
				}
				if added {
					sightings++
				}
			}
		}
	}
	log.Info("opensky scan done", "notable_new", sightings)
	return nil
}

type airState struct {
	icao24, callsign, country, squawk string
	lat, lon                          *float64
	altitude, velocity                *float32
	onGround                          bool
}

func notable(st airState) string {
	cs := strings.TrimSpace(st.callsign)
	for _, p := range watchlistCallsigns {
		if strings.HasPrefix(cs, p) {
			return "watchlist-callsign"
		}
	}
	switch st.squawk {
	case "7500", "7600", "7700":
		return "emergency-squawk"
	}
	// EU airspace is closed to Russian and Belarusian aircraft; any inside
	// the border sectors (which include international Baltic waters) is
	// worth a look.
	if st.country == "Russia" || st.country == "Belarus" {
		if !st.onGround {
			return "ru-by-aircraft"
		}
	}
	return ""
}

func (o *OpenSky) fetchToken(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {o.ClientID},
		"client_secret": {o.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("token response: status %d", resp.StatusCode)
	}
	return out.AccessToken, nil
}

func (o *OpenSky) states(ctx context.Context, token string, box Box) ([]airState, error) {
	q := url.Values{
		"lamin": {fmt.Sprint(box.LatMin)},
		"lomin": {fmt.Sprint(box.LonMin)},
		"lamax": {fmt.Sprint(box.LatMax)},
		"lomax": {fmt.Sprint(box.LonMax)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://opensky-network.org/api/states/all?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return parseStates(data)
}

// parseStates decodes the states/all response: {"time": ..., "states": [[...], ...]}
// with positional fields documented by OpenSky.
func parseStates(data []byte) ([]airState, error) {
	var raw struct {
		States [][]any `json:"states"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	str := func(v any) string {
		s, _ := v.(string)
		return s
	}
	f64 := func(v any) *float64 {
		if n, ok := v.(float64); ok {
			return &n
		}
		return nil
	}
	f32 := func(v any) *float32 {
		if n, ok := v.(float64); ok {
			m := float32(n)
			return &m
		}
		return nil
	}
	var out []airState
	for _, s := range raw.States {
		if len(s) < 15 {
			continue
		}
		onGround, _ := s[8].(bool)
		out = append(out, airState{
			icao24:   str(s[0]),
			callsign: str(s[1]),
			country:  str(s[2]),
			lon:      f64(s[5]),
			lat:      f64(s[6]),
			altitude: f32(s[7]),
			onGround: onGround,
			velocity: f32(s[9]),
			squawk:   str(s[14]),
		})
	}
	return out, nil
}
