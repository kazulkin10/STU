package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// AgentClient sends reports to Python moderation-agent.
type AgentClient struct {
	url    string
	client *http.Client
	logger zerolog.Logger
}

// NewAgentClient builds AI client pointing to moderation-agent.
func NewAgentClient(url string, logger zerolog.Logger) AIClient {
	if url == "" {
		return nil
	}
	return &AgentClient{
		url:    url,
		client: &http.Client{Timeout: 15 * time.Second},
		logger: logger,
	}
}

type agentRequest struct {
	ReportID       string `json:"report_id"`
	ReporterUserID string `json:"reporter_user_id"`
	ReportedUserID string `json:"reported_user_id"`
	Reason         string `json:"reason"`
	MessageText    string `json:"message_text"`
}

type agentResponse struct {
	Verdict    string  `json:"verdict"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Notes      string  `json:"notes"`
}

func (c *AgentClient) Analyze(ctx context.Context, input AIInput) (AIResult, error) {
	payload := agentRequest{
		ReportID:       input.ReportID.String(),
		ReporterUserID: input.ReporterID.String(),
		ReportedUserID: input.ReportedUserID.String(),
		Reason:         input.Reason,
		MessageText:    input.MessageText,
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(buf))
	if err != nil {
		return AIResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return AIResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return AIResult{}, fmt.Errorf("agent status %d", resp.StatusCode)
	}
	var parsed agentResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return AIResult{}, err
	}
	return AIResult{
		Verdict:    parsed.Verdict,
		Confidence: parsed.Confidence,
		Notes:      parsed.Notes,
	}, nil
}
