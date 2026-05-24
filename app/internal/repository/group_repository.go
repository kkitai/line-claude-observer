package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kkitai/line-claude-observer/app/internal/domain"
)

type GroupRepository interface {
	FindOrCreate(ctx context.Context, lineGroupID, sourceType, name string) (*domain.Group, error)
}

type pgGroupRepository struct {
	db *pgxpool.Pool
}

func NewGroupRepository(db *pgxpool.Pool) GroupRepository {
	return &pgGroupRepository{db: db}
}

func (r *pgGroupRepository) FindOrCreate(ctx context.Context, lineGroupID, sourceType, name string) (*domain.Group, error) {
	const q = `
		INSERT INTO groups (line_group_id, source_type, name)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (line_group_id) DO UPDATE SET name = NULLIF(EXCLUDED.name, '')
		RETURNING id, line_group_id, source_type, COALESCE(name, ''), created_at
	`
	g := &domain.Group{}
	err := r.db.QueryRow(ctx, q, lineGroupID, sourceType, name).Scan(
		&g.ID, &g.LineGroupID, &g.SourceType, &g.Name, &g.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("group FindOrCreate: %w", err)
	}
	return g, nil
}
