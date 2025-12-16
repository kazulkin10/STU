package dialogs

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"stu/internal/auth"
)

var (
	ErrForbidden = errors.New("forbidden")
)

// Service encapsulates dialog/message operations.
type Service struct {
	repo        Repository
	userFetcher func(ctx context.Context, email string) (auth.User, error)
	publisher   EventPublisher
}

func NewService(repo Repository, userFetcher func(ctx context.Context, email string) (auth.User, error)) *Service {
	return &Service{repo: repo, userFetcher: userFetcher}
}

func (s *Service) WithPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

func (s *Service) SetPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

// CreateDirect creates or returns existing direct dialog.
func (s *Service) CreateDirect(ctx context.Context, currentUser uuid.UUID, targetEmailOrID string) (uuid.UUID, error) {
	var peerID uuid.UUID
	if id, err := uuid.Parse(targetEmailOrID); err == nil {
		peerID = id
	} else {
		user, err := s.userFetcher(ctx, targetEmailOrID)
		if err != nil {
			return uuid.Nil, err
		}
		peerID = user.ID
	}
	if peerID == uuid.Nil || peerID == currentUser {
		return uuid.Nil, errors.New("invalid peer")
	}
	id, err := s.repo.GetOrCreateDirect(ctx, currentUser, peerID)
	if err == ErrDialogExist {
		return id, nil
	}
	return id, err
}

func (s *Service) ListDialogs(ctx context.Context, currentUser uuid.UUID, limit int) ([]Dialog, error) {
	return s.repo.ListDialogs(ctx, currentUser, limit)
}

func (s *Service) SendMessage(ctx context.Context, currentUser uuid.UUID, dialogID uuid.UUID, text string) (Message, error) {
	ok, err := s.repo.CheckMember(ctx, dialogID, currentUser)
	if err != nil {
		return Message{}, err
	}
	if !ok {
		return Message{}, ErrForbidden
	}
	id, created, err := s.repo.SaveMessage(ctx, dialogID, currentUser, text)
	if err != nil {
		return Message{}, err
	}
	msg := Message{ID: id, DialogID: dialogID, SenderID: currentUser, Text: text, CreatedAt: created, DeliveredPeer: false, ReadPeer: false}
	if s.publisher != nil {
		if members, err := s.repo.Members(ctx, dialogID); err == nil {
			_ = s.publisher.PublishMessage(ctx, msg, members)
		}
	}
	return msg, nil
}

func (s *Service) ListMessages(ctx context.Context, currentUser uuid.UUID, dialogID uuid.UUID, limit int, before int64) ([]Message, error) {
	ok, err := s.repo.CheckMember(ctx, dialogID, currentUser)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	return s.repo.ListMessages(ctx, dialogID, currentUser, limit, before)
}

func (s *Service) MarkDelivered(ctx context.Context, currentUser uuid.UUID, dialogID uuid.UUID, messageID int64) error {
	ok, err := s.repo.CheckMember(ctx, dialogID, currentUser)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if err := s.repo.MarkDelivered(ctx, dialogID, currentUser, messageID); err != nil {
		return err
	}
	if s.publisher != nil {
		if members, err := s.repo.Members(ctx, dialogID); err == nil {
			_ = s.publisher.PublishDelivery(ctx, dialogID, currentUser, messageID, members)
		}
	}
	return nil
}

func (s *Service) MarkRead(ctx context.Context, currentUser uuid.UUID, dialogID uuid.UUID, messageID int64) error {
	ok, err := s.repo.CheckMember(ctx, dialogID, currentUser)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if err := s.repo.MarkRead(ctx, dialogID, currentUser, messageID); err != nil {
		return err
	}
	if s.publisher != nil {
		if members, err := s.repo.Members(ctx, dialogID); err == nil {
			_ = s.publisher.PublishRead(ctx, dialogID, currentUser, messageID, members)
		}
	}
	return nil
}
