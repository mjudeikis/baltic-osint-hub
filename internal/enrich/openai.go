package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// openAIClient is a minimal Chat Completions client; the payload is simple
// enough that the official SDK would only add a dependency.
type openAIClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func newOpenAIClient(apiKey, baseURL string) *openAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &openAIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Minute},
	}
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
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("openai: status %d: %.200s", resp.StatusCode, data)
	}
	if out.Error != nil {
		return "", fmt.Errorf("openai: %s (%s)", out.Error.Message, out.Error.Type)
	}
	if resp.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: status %d, %d choices", resp.StatusCode, len(out.Choices))
	}
	return out.Choices[0].Message.Content, nil
}
