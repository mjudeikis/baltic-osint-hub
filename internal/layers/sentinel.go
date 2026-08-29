package layers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Sentinel runs Sentinel-1 SAR change detection over the monitored AOIs using
// the Copernicus Data Space Sentinel Hub Statistical API. The heavy raster
// work happens server-side: we send an evalscript and receive per-interval
// aggregates, so nothing here decodes imagery.
type Sentinel struct {
	ClientID     string
	ClientSecret string
	Client       *http.Client

	// Cached bearer token. Copernicus tokens are short-lived and a full pass
	// over the watchlist outlives one, so it is refreshed mid-run rather than
	// fetched once — otherwise every site after the expiry fails with 401.
	token       string
	tokenExpiry time.Time
}

const (
	cdseTokenURL = "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token"
	cdseStatsURL = "https://sh.dataspace.copernicus.eu/api/v1/statistics"

	// brightThresholdDB: bare soil and grass sit near -15 dB, forest around
	// -8 dB; metal vehicles, aircraft and rolling stock are corner reflectors
	// that return well above -5 dB.
	brightThresholdDB = -5.0

	// A single orbit direction keeps incidence angle comparable across the
	// series — mixing ascending and descending passes is the classic way to
	// manufacture false anomalies.
	orbitDirection = "DESCENDING"

	sarWindowDays = 180
	// Trailing window re-requested on every run after the initial backfill.
	sarOverlapDays = 18
	sarInterval    = "P6D" // Sentinel-1 revisit over the region
	sarResolutionM = 20    // metres; halves processing units vs native 10 m

	// Mean metres per degree of latitude. Longitude degrees shrink with
	// cos(latitude), so the two axes need different steps.
	metresPerDegreeLat = 111320.0
)

// resolutionDegrees converts a ground resolution in metres into the CRS units
// the Statistical API expects — degrees, because the bounds are sent as
// EPSG:4326. Passing metres directly makes the service read them as degrees
// and collapse the AOI to a single pixel.
func resolutionDegrees(metres, centreLat float64) (resx, resy float64) {
	resy = metres / metresPerDegreeLat
	resx = metres / (metresPerDegreeLat * math.Cos(centreLat*math.Pi/180))
	return resx, resy
}

// evalscript emits two outputs per pixel: a bright/not-bright mask (whose
// mean over the AOI is the bright-pixel fraction) and VV in decibels.
var evalscript = fmt.Sprintf(`//VERSION=3
function setup() {
  return {
    input: [{ bands: ["VV", "dataMask"] }],
    output: [
      { id: "bright", bands: 1, sampleType: "FLOAT32" },
      { id: "vvdb", bands: 1, sampleType: "FLOAT32" },
      { id: "dataMask", bands: 1 }
    ]
  };
}
function evaluatePixel(s) {
  var db = 10 * Math.log10(Math.max(s.VV, 1e-7));
  return {
    bright: [db > %.1f ? 1 : 0],
    vvdb: [db],
    dataMask: [s.dataMask]
  };
}`, brightThresholdDB)

func (s *Sentinel) token2(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {s.ClientID},
		"client_secret": {s.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdseTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("token decode: status %d", resp.StatusCode)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token: status %d: %s", resp.StatusCode, out.Error)
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 10 * time.Minute // conservative default if the field is absent
	}
	s.token = out.AccessToken
	s.tokenExpiry = time.Now().Add(ttl)
	return out.AccessToken, nil
}

// validToken returns a token good for at least another minute, refreshing when
// the cached one is close to expiry.
func (s *Sentinel) validToken(ctx context.Context) (string, error) {
	if s.token != "" && time.Now().Before(s.tokenExpiry.Add(-time.Minute)) {
		return s.token, nil
	}
	return s.token2(ctx)
}

// Run refreshes every AOI's time series and re-evaluates its anomaly state.
func (s *Sentinel) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	if _, err := s.validToken(ctx); err != nil {
		return err
	}
	to := time.Now().UTC().Truncate(24 * time.Hour)

	var failures []string
	for _, aoi := range MonitoredAOIs {
		// Only the first run needs the full baseline window. Afterwards ask
		// for a short trailing window — re-fetching 180 days per AOI per day
		// would burn the processing-unit budget for data already stored. The
		// overlap re-covers intervals whose passes may have landed late.
		from := to.AddDate(0, 0, -sarWindowDays)
		if latest, ok, err := db.SARLatestInterval(ctx, aoi.Key); err != nil {
			return err
		} else if ok {
			from = latest.AddDate(0, 0, -sarOverlapDays)
		}

		token, err := s.validToken(ctx)
		if err != nil {
			return err
		}
		obs, err := s.statistics(ctx, token, aoi, from, to)
		if err != nil {
			log.Warn("sar aoi failed", "aoi", aoi.Key, "err", err)
			failures = append(failures, aoi.Key)
			continue
		}
		for i := range obs {
			if err := db.UpsertSAR(ctx, aoi.Key, &obs[i]); err != nil {
				return err
			}
		}
		// Re-read the stored series so the verdict covers observations from
		// earlier runs too, not just this response window.
		series, err := db.SARSeries(ctx, aoi.Key)
		if err != nil {
			return err
		}
		a := DetectAnomaly(series)
		if a.Detected {
			latest := series[len(series)-1]
			added, err := db.InsertSARAnomaly(ctx, aoi.Key, latest.Start, a.Latest, a.Median, a.ZScore)
			if err != nil {
				return err
			}
			if added {
				log.Info("sar anomaly", "aoi", aoi.Key, "latest", a.Latest,
					"median", a.Median, "z", a.ZScore)
			}
		}
		log.Info("sar aoi updated", "aoi", aoi.Key, "intervals", len(obs),
			"series", len(series), "anomaly", a.Detected)
	}
	if len(failures) > 0 {
		// Any failure keeps the layer's gate open so the next run continues
		// where this one stopped. Sites already stored are re-requested with
		// only a short trailing window, so successive runs get further.
		return fmt.Errorf("%d of %d AOIs incomplete: %s",
			len(failures), len(MonitoredAOIs), strings.Join(failures, ", "))
	}
	return nil
}

type statsRequest struct {
	Input struct {
		Bounds struct {
			BBox       []float64      `json:"bbox"`
			Properties map[string]any `json:"properties"`
		} `json:"bounds"`
		Data []map[string]any `json:"data"`
	} `json:"input"`
	Aggregation  map[string]any `json:"aggregation"`
	Calculations map[string]any `json:"calculations"`
}

type statsResponse struct {
	Data []struct {
		Interval struct {
			From time.Time `json:"from"`
			To   time.Time `json:"to"`
		} `json:"interval"`
		Outputs map[string]struct {
			Bands map[string]struct {
				Stats struct {
					Mean        float64 `json:"mean"`
					SampleCount int64   `json:"sampleCount"`
					NoDataCount int64   `json:"noDataCount"`
				} `json:"stats"`
			} `json:"bands"`
		} `json:"outputs"`
		Error any `json:"error"`
	} `json:"data"`
	Status string `json:"status"`
	Error  any    `json:"error"`
}

func (s *Sentinel) statistics(ctx context.Context, token string, aoi AOI, from, to time.Time) ([]store.SARObservation, error) {
	var body statsRequest
	body.Input.Bounds.BBox = []float64{aoi.Box.LonMin, aoi.Box.LatMin, aoi.Box.LonMax, aoi.Box.LatMax}
	body.Input.Bounds.Properties = map[string]any{
		"crs": "http://www.opengis.net/def/crs/EPSG/0/4326",
	}
	body.Input.Data = []map[string]any{{
		"type": "sentinel-1-grd",
		"dataFilter": map[string]any{
			"acquisitionMode": "IW",
			"polarization":    "DV",
			"orbitDirection":  orbitDirection,
			"resolution":      "HIGH",
		},
		"processing": map[string]any{
			"orthorectify": true,
			"backCoeff":    "SIGMA0_ELLIPSOID",
		},
	}}
	centreLat, _ := aoi.Centre()
	resx, resy := resolutionDegrees(sarResolutionM, centreLat)
	body.Aggregation = map[string]any{
		"timeRange": map[string]string{
			"from": from.Format(time.RFC3339),
			"to":   to.Format(time.RFC3339),
		},
		"aggregationInterval": map[string]string{"of": sarInterval},
		"evalscript":          evalscript,
		"resx":                resx,
		"resy":                resy,
	}
	body.Calculations = map[string]any{"default": map[string]any{}}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdseStatsURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("statistics: status %d: %.200s", resp.StatusCode, data)
	}
	return parseStatistics(data)
}

// parseStatistics turns the Statistical API response into observations,
// skipping intervals the service could not compute (no acquisition, clouds of
// the radar world: missing passes).
func parseStatistics(data []byte) ([]store.SARObservation, error) {
	var out statsResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("statistics decode: %w", err)
	}
	var obs []store.SARObservation
	for _, d := range out.Data {
		if d.Error != nil {
			continue
		}
		bright, ok := d.Outputs["bright"]
		if !ok {
			continue
		}
		band, ok := firstBand(bright.Bands)
		if !ok || band.Stats.SampleCount == 0 {
			continue
		}
		// A pass that barely clips the AOI carries no useful aggregate.
		valid := band.Stats.SampleCount - band.Stats.NoDataCount
		if valid <= 0 || float64(valid)/float64(band.Stats.SampleCount) < 0.5 {
			continue
		}
		o := store.SARObservation{
			Start:          d.Interval.From,
			End:            d.Interval.To,
			BrightFraction: band.Stats.Mean,
			SampleCount:    valid,
		}
		if vv, ok := d.Outputs["vvdb"]; ok {
			if vb, ok := firstBand(vv.Bands); ok {
				o.MeanDB = vb.Stats.Mean
			}
		}
		obs = append(obs, o)
	}
	return obs, nil
}

type bandStats = struct {
	Stats struct {
		Mean        float64 `json:"mean"`
		SampleCount int64   `json:"sampleCount"`
		NoDataCount int64   `json:"noDataCount"`
	} `json:"stats"`
}

// firstBand returns the single band of a one-band output regardless of the
// name the service assigns it (B0 today, but not contractually).
func firstBand(bands map[string]bandStats) (bandStats, bool) {
	for _, b := range bands {
		return b, true
	}
	return bandStats{}, false
}
