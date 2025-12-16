package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"stu/internal/config"
)

// TimewebAIClient calls Timeweb Cloud agent for moderation.
type TimewebAIClient struct {
	url    string
	apiKey string
	access string
	client *http.Client
	logger zerolog.Logger
}

// NewTimewebAIClient returns AI client or nil if API key missing.
func NewTimewebAIClient(cfg config.TimewebAgentConfig, logger zerolog.Logger) AIClient {
	if cfg.APIKey == "" {
		return nil
	}
	return &TimewebAIClient{
		url:    cfg.URL,
		apiKey: cfg.APIKey,
		access: cfg.AccessID,
		client: &http.Client{Timeout: 12 * time.Second},
		logger: logger,
	}
}

type timewebMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type timewebRequest struct {
	Messages  []timewebMessage `json:"messages"`
	MaxTokens int              `json:"max_tokens,omitempty"`
}

type timewebResponse struct {
	Choices []struct {
		Message timewebMessage `json:"message"`
	} `json:"choices"`
}

// Analyze sends moderation request to Timeweb agent.
func (c *TimewebAIClient) Analyze(ctx context.Context, input AIInput) (AIResult, error) {
	systemPrompt := `Ты модератор. Верни JSON строго вида {"verdict":"allow|needs_review|ban_suspected","category":"spam|fraud|extremism|csam|hate|other","confidence":0..1,"notes":"кратко по-русски"}. Не добавляй другого текста.`
	userContent := fmt.Sprintf("Репорт ID %s. Жалуется пользователь %s на %s. Причина: %s. Текст сообщения: %s",
		input.ReportID, input.ReporterID, input.ReportedUserID, sanitize(input.Reason), sanitize(input.MessageText))

	reqBody := timewebRequest{
		Messages: []timewebMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		MaxTokens: 200,
	}
	buf, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(buf))
	if err != nil {
		return AIResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.access != "" {
		httpReq.Header.Set("X-Access-Id", c.access)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return AIResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return AIResult{}, fmt.Errorf("timeweb status %d", resp.StatusCode)
	}
	var twResp timewebResponse
	if err := json.NewDecoder(resp.Body).Decode(&twResp); err != nil {
		return AIResult{}, err
	}
	if len(twResp.Choices) == 0 {
		return AIResult{}, fmt.Errorf("empty response")
	}
	raw := twResp.Choices[0].Message.Content
	var parsed struct {
		Verdict    string  `json:"verdict"`
		Category   string  `json:"category"`
		Confidence float64 `json:"confidence"`
		Notes      string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// try to trim code fences if present
		raw = strings.Trim(raw, "` \n")
		raw = strings.TrimPrefix(raw, "json")
		if err2 := json.Unmarshal([]byte(raw), &parsed); err2 != nil {
			return AIResult{}, err
		}
	}
	notes := parsed.Notes
	if notes == "" {
		notes = parsed.Category
	}
	return AIResult{
		Verdict:    parsed.Verdict,
		Confidence: parsed.Confidence,
		Notes:      notes,
	}, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
