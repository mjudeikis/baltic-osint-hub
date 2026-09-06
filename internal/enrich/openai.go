package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// openAIClient is a minimal Chat Completions client; the payload is simple
// enough that the official SDK would only add a dependency.
type openAIClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
	log     *slog.Logger
	// retryBase is the first backoff delay; tests shorten it.
	retryBase time.Duration
}

func newOpenAIClient(apiKey, baseURL string) *openAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &openAIClient{
		apiKey:    apiKey,
		baseURL:   baseURL,
		http:      &http.Client{Timeout: 3 * time.Minute},
		log:       slog.Default(),
		retryBase: 2 * time.Second,
	}
}

// ErrTruncated reports that the model hit max_completion_tokens before it
// finished (finish_reason "length"). The reply is then a cut-off JSON array,
// and the caller must not retry the same batch — it will be cut off again —
// but split it or retire it.
var ErrTruncated = errors.New("openai: response truncated")

// maxTries bounds retries on 429 and 5xx. Three is one original request plus
// two retries: enough for a rate-limit blip, not so many that a real outage
// keeps the collector's run budget busy.
const maxTries = 3

// retryable reports whether a status is worth another attempt: rate limits
// and server-side failures are; 4xx from our own request never are.
func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	// max_completion_tokens is the current name; the older max_tokens is
	// rejected by reasoning models (gpt-5 family).
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
	// Seed pins sampling so the same article classifies the same way across
	// runs and across environments.
	//
	// This exists because dev and prod classified one identical article — an
	// AP report on Patriot stocks — at severity 4 and severity 3 respectively,
	// which put the published regional posture two levels apart: High in one,
	// Watchful in the other.
	//
	// Best-effort, not a guarantee. OpenAI reproduces results only while the
	// backend is unchanged, and temperature cannot be pinned alongside it —
	// gpt-5-mini rejects any value but the default. So this removes
	// environment-to-environment drift without making classification
	// deterministic; the corroboration requirement in internal/posture is what
	// actually stops one uncertain classification from moving the reading.
	Seed int `json:"seed,omitempty"`
}

// classifierSeed is arbitrary but fixed; only its stability matters.
const classifierSeed = 20260829

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	// Usage is logged per call so spend is visible per run rather than only
	// on the monthly invoice.
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// complete runs one chat completion, retrying rate limits and server errors
// with jittered exponential backoff.
func (c *openAIClient) complete(ctx context.Context, model, system, user string, maxTokens int) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxCompletionTokens: maxTokens,
		Seed:                classifierSeed,
	})
	if err != nil {
		return "", err
	}
	var lastErr error
	for try := 0; try < maxTries; try++ {
		if try > 0 {
			// Full jitter on a doubling base, so several collectors hitting
			// the same limit do not retry in lockstep.
			delay := c.retryBase << (try - 1)
			delay += time.Duration(rand.Int64N(int64(delay) + 1))
			c.log.Warn("openai: retrying", "try", try+1, "in", delay.String(), "err", lastErr)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		text, status, err := c.once(ctx, model, body)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retryable(status) {
			return "", err
		}
	}
	return "", lastErr
}

// once performs a single request. The returned status is 0 for transport
// failures, which are retried like a 5xx: a dropped connection mid-response
// is the same class of problem as the server giving up.
func (c *openAIClient) once(ctx context.Context, model string, body []byte) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", 0, err
		}
		return "", http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", resp.StatusCode, fmt.Errorf("openai: status %d: %.200s", resp.StatusCode, data)
	}
	if out.Error != nil {
		return "", resp.StatusCode, fmt.Errorf("openai: status %d: %s (%s)", resp.StatusCode, out.Error.Message, out.Error.Type)
	}
	if resp.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return "", resp.StatusCode, fmt.Errorf("openai: status %d, %d choices", resp.StatusCode, len(out.Choices))
	}
	c.log.Info("openai usage", "model", model,
		"prompt_tokens", out.Usage.PromptTokens, "completion_tokens", out.Usage.CompletionTokens,
		"finish_reason", out.Choices[0].FinishReason)
	if out.Choices[0].FinishReason == "length" {
		return "", resp.StatusCode, ErrTruncated
	}
	return out.Choices[0].Message.Content, resp.StatusCode, nil
}
