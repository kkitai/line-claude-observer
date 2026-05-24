package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

func TestGroupRepository_FindOrCreate(t *testing.T) {
	truncateTables(t)
	repo := repository.NewGroupRepository(testPool)
	ctx := context.Background()

	t.Run("creates new group", func(t *testing.T) {
		g, err := repo.FindOrCreate(ctx, "C001", "group", "Test Group")
		require.NoError(t, err)
		assert.NotEmpty(t, g.ID)
		assert.Equal(t, "C001", g.LineGroupID)
		assert.Equal(t, "group", g.SourceType)
		assert.Equal(t, "Test Group", g.Name)
		assert.NotZero(t, g.CreatedAt)
	})

	t.Run("returns same ID on duplicate line_group_id", func(t *testing.T) {
		g1, err := repo.FindOrCreate(ctx, "C002", "room", "My Room")
		require.NoError(t, err)

		g2, err := repo.FindOrCreate(ctx, "C002", "room", "My Room Renamed")
		require.NoError(t, err)

		assert.Equal(t, g1.ID, g2.ID)
		assert.Equal(t, "My Room Renamed", g2.Name)
	})

	t.Run("handles empty name", func(t *testing.T) {
		g, err := repo.FindOrCreate(ctx, "U003", "user", "")
		require.NoError(t, err)
		assert.Equal(t, "", g.Name)
	})

	t.Run("handles different source types", func(t *testing.T) {
		_, err := repo.FindOrCreate(ctx, "R004", "room", "Room")
		require.NoError(t, err)

		_, err = repo.FindOrCreate(ctx, "U005", "user", "DM")
		require.NoError(t, err)
	})
}
