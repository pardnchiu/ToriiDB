package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/pardnchiu/go-utils/utils"
)

const (
	defaultModel = "text-embedding-3-small"
	defaultDim   = 256
	endpoint     = "https://api.openai.com/v1/embeddings"
	timeout      = 30 * time.Second
)

type Client struct {
	APIKey string
	Dim    int
	Model  string
	http   *http.Client
}

var (
	once     sync.Once
	instance *Client
	initErr  error
)

type request struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	Dimensions     int    `json:"dimensions,omitempty"`
	EncodingFormat string `json:"encoding_format,omitempty"`
}

type response struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func New() (*Client, error) {
	once.Do(func() {
		_ = godotenv.Load()

		apiKey := strings.TrimSpace(utils.GetWithDefault("OPENAI_API_KEY", ""))
		if apiKey == "" {
			initErr = errors.New("OPENAI_API_KEY is required")
			return
		}

		dim := utils.GetWithDefaultInt("TORIIDB_EMBED_DIM", defaultDim)
		if dim <= 0 {
			dim = defaultDim
		}
		instance = &Client{
			APIKey: apiKey,
			Dim:    dim,
			Model:  defaultModel,
			http:   &http.Client{Timeout: timeout},
		}
	})
	return instance, initErr
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text is empty")
	}

	body := request{
		Input:          text,
		Model:          c.Model,
		Dimensions:     c.Dim,
		EncodingFormat: "float",
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client.Do: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, string(rawResp))
	}

	var parsed response
	if err := json.Unmarshal(rawResp, &parsed); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, errors.New("response is empty")
	}

	if len(parsed.Data[0].Embedding) != c.Dim {
		return nil, fmt.Errorf("dim is mismatch: expected %d, got %d", c.Dim, len(parsed.Data[0].Embedding))
	}
	return parsed.Data[0].Embedding, nil
}
