package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kkitai/line-claude-observer/app/internal/domain"
)

type MessageRepository interface {
	Save(ctx context.Context, msg *domain.Message) error
	FindRecent(ctx context.Context, groupID uuid.UUID, limit int) ([]domain.Message, error)
}

type pgMessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) MessageRepository {
	return &pgMessageRepository{db: db}
}

func (r *pgMessageRepository) Save(ctx context.Context, msg *domain.Message) error {
	const q = `
		INSERT INTO messages (group_id, line_user_id, display_name, message_type, content, received_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, q,
		msg.GroupID,
		msg.LineUserID,
		msg.DisplayName,
		msg.MessageType,
		msg.Content,
		msg.ReceivedAt,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("message Save: %w", err)
	}
	return nil
}

func (r *pgMessageRepository) FindRecent(ctx context.Context, groupID uuid.UUID, limit int) ([]domain.Message, error) {
	const q = `
		SELECT id, group_id, line_user_id, display_name, message_type, COALESCE(content, ''), received_at, created_at
		FROM messages
		WHERE group_id = $1
		ORDER BY received_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, q, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("message FindRecent: %w", err)
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(
			&m.ID, &m.GroupID, &m.LineUserID, &m.DisplayName,
			&m.MessageType, &m.Content, &m.ReceivedAt, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("message FindRecent scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("message FindRecent rows: %w", err)
	}
	return msgs, nil
}
