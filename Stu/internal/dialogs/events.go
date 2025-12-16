package dialogs

import (
	"context"

	"github.com/google/uuid"
)

// EventPublisher pushes dialog events to realtime.
type EventPublisher interface {
	PublishMessage(ctx context.Context, msg Message, members []uuid.UUID) error
	PublishDelivery(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64, members []uuid.UUID) error
	PublishRead(ctx context.Context, dialogID uuid.UUID, userID uuid.UUID, messageID int64, members []uuid.UUID) error
}
