package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkitai/line-claude-observer/app/internal/domain"
	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

func TestMessageRepository_Save(t *testing.T) {
	truncateTables(t)
	groupRepo := repository.NewGroupRepository(testPool)
	msgRepo := repository.NewMessageRepository(testPool)
	ctx := context.Background()

	group, err := groupRepo.FindOrCreate(ctx, "G_SAVE_001", "group", "Save Test Group")
	require.NoError(t, err)

	t.Run("saves text message and populates ID and CreatedAt", func(t *testing.T) {
		msg := &domain.Message{
			GroupID:     group.ID,
			LineUserID:  "U001",
			DisplayName: "Alice",
			MessageType: "text",
			Content:     "Hello!",
			ReceivedAt:  time.Now(),
		}
		err := msgRepo.Save(ctx, msg)
		require.NoError(t, err)
		assert.NotEmpty(t, msg.ID)
		assert.NotZero(t, msg.CreatedAt)
	})

	t.Run("saves non-text message with empty content", func(t *testing.T) {
		msg := &domain.Message{
			GroupID:     group.ID,
			LineUserID:  "U001",
			DisplayName: "Alice",
			MessageType: "image",
			Content:     "",
			ReceivedAt:  time.Now(),
		}
		err := msgRepo.Save(ctx, msg)
		require.NoError(t, err)
		assert.NotEmpty(t, msg.ID)
	})

	t.Run("saves sticker message with empty content", func(t *testing.T) {
		msg := &domain.Message{
			GroupID:     group.ID,
			LineUserID:  "U002",
			DisplayName: "Bob",
			MessageType: "sticker",
			Content:     "",
			ReceivedAt:  time.Now(),
		}
		err := msgRepo.Save(ctx, msg)
		require.NoError(t, err)
	})
}

func TestMessageRepository_FindRecent(t *testing.T) {
	truncateTables(t)
	groupRepo := repository.NewGroupRepository(testPool)
	msgRepo := repository.NewMessageRepository(testPool)
	ctx := context.Background()

	group, err := groupRepo.FindOrCreate(ctx, "G_FIND_001", "group", "Find Test Group")
	require.NoError(t, err)

	baseTime := time.Now().Truncate(time.Millisecond).Add(-10 * time.Minute)
	for i := 0; i < 5; i++ {
		msg := &domain.Message{
			GroupID:     group.ID,
			LineUserID:  "U001",
			DisplayName: "Alice",
			MessageType: "text",
			Content:     fmt.Sprintf("Message %d", i+1),
			ReceivedAt:  baseTime.Add(time.Duration(i) * time.Minute),
		}
		require.NoError(t, msgRepo.Save(ctx, msg))
	}

	t.Run("returns N most recent messages in DESC order", func(t *testing.T) {
		msgs, err := msgRepo.FindRecent(ctx, group.ID, 3)
		require.NoError(t, err)
		assert.Len(t, msgs, 3)
		assert.Equal(t, "Message 5", msgs[0].Content)
		assert.Equal(t, "Message 4", msgs[1].Content)
		assert.Equal(t, "Message 3", msgs[2].Content)
	})

	t.Run("returns all messages when limit exceeds count", func(t *testing.T) {
		msgs, err := msgRepo.FindRecent(ctx, group.ID, 20)
		require.NoError(t, err)
		assert.Len(t, msgs, 5)
	})

	t.Run("returns empty slice for group with no messages", func(t *testing.T) {
		newGroup, err := groupRepo.FindOrCreate(ctx, "G_EMPTY_001", "group", "Empty Group")
		require.NoError(t, err)

		msgs, err := msgRepo.FindRecent(ctx, newGroup.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, msgs)
	})

	t.Run("populates all fields correctly", func(t *testing.T) {
		msgs, err := msgRepo.FindRecent(ctx, group.ID, 1)
		require.NoError(t, err)
		require.Len(t, msgs, 1)

		m := msgs[0]
		assert.NotEmpty(t, m.ID)
		assert.Equal(t, group.ID, m.GroupID)
		assert.Equal(t, "U001", m.LineUserID)
		assert.Equal(t, "Alice", m.DisplayName)
		assert.Equal(t, "text", m.MessageType)
		assert.NotZero(t, m.ReceivedAt)
		assert.NotZero(t, m.CreatedAt)
	})
}
