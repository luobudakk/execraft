package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	Chat(ctx context.Context, model string, messages []Message) (string, error)
}

type Config struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
}

type Runtime struct {
	provider Provider
	cfg      Config
}

func NewRuntime(cfg Config) (*Runtime, error) {
	p := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch p {
	case "", "mock":
		return &Runtime{provider: mockProvider{}, cfg: cfg}, nil
	case "ollama":
		base := cfg.BaseURL
		if strings.TrimSpace(base) == "" {
			base = "http://127.0.0.1:11434"
		}
		return &Runtime{provider: ollamaProvider{baseURL: strings.TrimRight(base, "/"), client: &http.Client{Timeout: 45 * time.Second}}, cfg: cfg}, nil
	case "openai_compat":
		base := cfg.BaseURL
		if strings.TrimSpace(base) == "" {
			base = "https://api.openai.com/v1"
		}
		return &Runtime{provider: openAICompatProvider{baseURL: strings.TrimRight(base, "/"), apiKey: strings.TrimSpace(cfg.APIKey), client: &http.Client{Timeout: 45 * time.Second}}, cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.Provider)
	}
}

func (r *Runtime) Chat(ctx context.Context, messages []Message) (string, error) {
	model := strings.TrimSpace(r.cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	return r.provider.Chat(ctx, model, messages)
}

type mockProvider struct{}

func (m mockProvider) Chat(_ context.Context, model string, messages []Message) (string, error) {
	last := ""
	if len(messages) > 0 {
		last = messages[len(messages)-1].Content
	}
	return fmt.Sprintf("[mock:%s] plan suggestion based on: %s", model, truncate(last, 280)), nil
}

type openAICompatProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (o openAICompatProvider) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if o.apiKey == "" {
		return "", fmt.Errorf("llm api key is required for openai_compat")
	}
	reqBody := map[string]any{"model": model, "messages": messages, "stream": false}
	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("openai_compat status %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai_compat empty choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

type ollamaProvider struct {
	baseURL string
	client  *http.Client
}

func (o ollamaProvider) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	reqBody := map[string]any{"model": model, "stream": false, "messages": messages}
	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	return parsed.Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
