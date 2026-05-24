package domain

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID          uuid.UUID
	LineGroupID string
	SourceType  string
	Name        string
	CreatedAt   time.Time
}
