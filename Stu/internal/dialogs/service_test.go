package dialogs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"stu/internal/auth"
)

type memRepo struct {
	dialogMembers map[uuid.UUID][]uuid.UUID
	messages      map[uuid.UUID][]Message
}

func newMemRepo() *memRepo {
	return &memRepo{
		dialogMembers: make(map[uuid.UUID][]uuid.UUID),
		messages:      make(map[uuid.UUID][]Message),
	}
}

func (m *memRepo) CreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	m.dialogMembers[id] = []uuid.UUID{initiator, peer}
	return id, nil
}

func (m *memRepo) GetOrCreateDirect(ctx context.Context, initiator uuid.UUID, peer uuid.UUID) (uuid.UUID, error) {
	for id, members := range m.dialogMembers {
		if contains(members, initiator) && contains(members, peer) && len(members) == 2 {
			return id, ErrDialogExist
		}
	}
	return m.CreateDirect(ctx, initiator, peer)
}

func (m *memRepo) ListDialogs(ctx context.Context, userID uuid.UUID, limit int) ([]Dialog, error) {
	var res []Dialog
	for id, members := range m.dialogMembers {
		if contains(members, userID) {
			msgs := m.messages[id]
			var last *Message
			if len(msgs) > 0 {
				lm := msgs[len(msgs)-1]
				last = &lm
			}
			res = append(res, Dialog{ID: id, LastMessage: last})
		}
	}
	return res, nil
}

func (m *memRepo) CheckMember(ctx context.Context, dialogID, userID uuid.UUID) (bool, error) {
	return contains(m.dialogMembers[dialogID], userID), nil
}

func (m *memRepo) SaveMessage(ctx context.Context, dialogID, sender uuid.UUID, text string) (int64, time.Time, error) {
	msg := Message{ID: int64(len(m.messages[dialogID]) + 1), DialogID: dialogID, SenderID: sender, Text: text, CreatedAt: time.Now()}
	m.messages[dialogID] = append(m.messages[dialogID], msg)
	return msg.ID, msg.CreatedAt, nil
}

func (m *memRepo) ListMessages(ctx context.Context, dialogID, userID uuid.UUID, limit int, before int64) ([]Message, error) {
	msgs := m.messages[dialogID]
	return msgs, nil
}

func (m *memRepo) MarkDelivered(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error {
	return nil
}

func (m *memRepo) MarkRead(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64) error {
	return nil
}

func contains(list []uuid.UUID, id uuid.UUID) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

func (m *memRepo) Members(ctx context.Context, dialogID uuid.UUID) ([]uuid.UUID, error) {
	return m.dialogMembers[dialogID], nil
}

func dummyFetcher(userID uuid.UUID) func(ctx context.Context, email string) (auth.User, error) {
	return func(ctx context.Context, email string) (auth.User, error) {
		return auth.User{ID: userID, Email: email, IsActive: true}, nil
	}
}

func TestCreateAndSendMessage(t *testing.T) {
	repo := newMemRepo()
	u1 := uuid.New()
	u2 := uuid.New()
	svc := NewService(repo, dummyFetcher(u2))

	dialogID, err := svc.CreateDirect(context.Background(), u1, u2.String())
	if err != nil {
		t.Fatalf("create direct: %v", err)
	}
	msg, err := svc.SendMessage(context.Background(), u1, dialogID, "hello")
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if msg.Text != "hello" || msg.SenderID != u1 {
		t.Fatalf("unexpected message: %+v", msg)
	}
	msgs, err := svc.ListMessages(context.Background(), u1, dialogID, 10, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestAccessControl(t *testing.T) {
	repo := newMemRepo()
	u1 := uuid.New()
	u2 := uuid.New()
	svc := NewService(repo, dummyFetcher(u2))
	dialogID, _ := svc.CreateDirect(context.Background(), u1, u2.String())

	// stranger
	if _, err := svc.SendMessage(context.Background(), uuid.New(), dialogID, "nope"); err != ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
