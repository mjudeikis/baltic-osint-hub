package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EmbedModel is used only for near-duplicate detection between English
// summaries, not for semantic search, so the small model is the right one.
const EmbedModel = "text-embedding-3-small"

// EmbedDims requests a shortened embedding. text-embedding-3-small is trained
// so that a truncated prefix remains usable, and 512 dimensions is ample for
// deciding whether two one-sentence summaries describe the same event. It also
// keeps the stored vector at ~2KB per incident instead of ~6KB.
const EmbedDims = 512

// Embedder produces vectors for clustering. It shares the OpenAI credentials
// with the classifier.
type Embedder struct {
	client *openAIClient
}

func NewEmbedder(apiKey, baseURL string) *Embedder {
	return &Embedder{client: newOpenAIClient(apiKey, baseURL)}
}

type embedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Embed returns one vector per input, in the same order. The API may return
// data out of order, so results are placed by their reported index.
func (e *Embedder) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	// The API rejects empty strings; substitute a placeholder so the caller's
	// indexing stays aligned with what it sent.
	in := make([]string, len(inputs))
	for i, s := range inputs {
		if s == "" {
			s = "(no summary)"
		}
		in[i] = s
	}

	body, err := json.Marshal(embedRequest{Model: EmbedModel, Input: in, Dimensions: EmbedDims})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.client.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.client.apiKey)

	resp, err := e.client.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var out embedResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("embeddings: status %d: %.200s", resp.StatusCode, data)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embeddings: %s (%s)", out.Error.Message, out.Error.Type)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode)
	}
	if len(out.Data) != len(in) {
		return nil, fmt.Errorf("embeddings: asked for %d vectors, got %d", len(in), len(out.Data))
	}

	vecs := make([][]float32, len(in))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vecs) {
			return nil, fmt.Errorf("embeddings: index %d out of range", d.Index)
		}
		vecs[d.Index] = d.Embedding
	}
	for i, v := range vecs {
		if len(v) == 0 {
			return nil, fmt.Errorf("embeddings: no vector for input %d", i)
		}
	}
	return vecs, nil
}
