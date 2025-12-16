package reports

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var ErrInvalidReport = errors.New("invalid report")

type Service struct {
	repo   Repository
	ai     AIClient
	logger zerolog.Logger
}

// AIInput describes payload sent to AI classifier.
type AIInput struct {
	ReportID       uuid.UUID
	ReporterID     uuid.UUID
	ReportedUserID uuid.UUID
	Reason         string
	MessageText    string
}

// AIResult is a normalized verdict from classifier.
type AIResult struct {
	Verdict    string
	Confidence float64
	Notes      string
}

// AIClient runs moderation classification.
type AIClient interface {
	Analyze(ctx context.Context, input AIInput) (AIResult, error)
}

func NewService(repo Repository, ai AIClient, logger zerolog.Logger) *Service {
	return &Service{repo: repo, ai: ai, logger: logger}
}

func (s *Service) Create(ctx context.Context, reporter uuid.UUID, payload CreateReport) (Report, error) {
	if payload.Reason == "" {
		return Report{}, ErrInvalidReport
	}
	if payload.ReportedUserID == uuid.Nil {
		return Report{}, ErrInvalidReport
	}
	if payload.DialogID == uuid.Nil && payload.MessageID == nil {
		return Report{}, ErrInvalidReport
	}
	rep := Report{
		ReporterID:     reporter,
		ReportedUserID: payload.ReportedUserID,
		DialogID:       ptrUUID(payload.DialogID),
		MessageID:      payload.MessageID,
		Reason:         payload.Reason,
		Status:         "open",
		CreatedAt:      time.Now(),
	}
	rep, err := s.repo.Create(ctx, rep)
	if err != nil {
		return Report{}, err
	}
	if s.ai != nil {
		go s.runAI(ctx, rep)
	}
	return rep, nil
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]Report, error) {
	return s.repo.ListMine(ctx, userID)
}

func (s *Service) ListAdmin(ctx context.Context, status string, limit, offset int) ([]ReportAdminView, error) {
	return s.repo.ListAdmin(ctx, status, limit, offset)
}

func (s *Service) runAI(parent context.Context, rep Report) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var msgText string
	if rep.MessageID != nil {
		if text, err := s.repo.GetMessageText(ctx, *rep.MessageID); err == nil {
			msgText = text
		}
	}
	input := AIInput{
		ReportID:       rep.ID,
		ReporterID:     rep.ReporterID,
		ReportedUserID: rep.ReportedUserID,
		Reason:         rep.Reason,
		MessageText:    msgText,
	}
	res, err := s.ai.Analyze(ctx, input)
	if err != nil {
		s.logger.Warn().Err(err).Msg("ai analysis failed")
		return
	}
	if err := s.repo.UpdateAIResult(ctx, rep.ID, res.Verdict, res.Confidence, res.Notes); err != nil {
		s.logger.Warn().Err(err).Msg("ai result save failed")
	}
}

func (s *Service) Close(ctx context.Context, id uuid.UUID) error {
	return s.repo.Close(ctx, id)
}

type CreateReport struct {
	ReportedUserID uuid.UUID
	DialogID       uuid.UUID
	MessageID      *int64
	Reason         string
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
