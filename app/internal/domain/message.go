package domain

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	LineUserID  string
	DisplayName string
	MessageType string
	Content     string
	ReceivedAt  time.Time
	CreatedAt   time.Time
}
