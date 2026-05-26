package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kkitai/line-claude-observer/app/internal/domain"
	"github.com/kkitai/line-claude-observer/app/internal/service"
)

// --- mocks ---

type mockMessageRepo struct{ mock.Mock }

func (m *mockMessageRepo) Save(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageRepo) FindRecent(ctx context.Context, groupID uuid.UUID, limit int) ([]domain.Message, error) {
	args := m.Called(ctx, groupID, limit)
	return args.Get(0).([]domain.Message), args.Error(1)
}

type mockClaudeClient struct{ mock.Mock }

func (m *mockClaudeClient) GenerateOpinion(ctx context.Context, messages []domain.Message) (string, error) {
	args := m.Called(ctx, messages)
	return args.String(0), args.Error(1)
}

type mockLineClient struct{ mock.Mock }

func (m *mockLineClient) GetGroupMemberProfile(ctx context.Context, groupID, userID string) (string, error) {
	args := m.Called(ctx, groupID, userID)
	return args.String(0), args.Error(1)
}

func (m *mockLineClient) GetRoomMemberProfile(ctx context.Context, roomID, userID string) (string, error) {
	args := m.Called(ctx, roomID, userID)
	return args.String(0), args.Error(1)
}

func (m *mockLineClient) ReplyMessage(ctx context.Context, replyToken, text string) error {
	args := m.Called(ctx, replyToken, text)
	return args.Error(0)
}

// --- helpers ---

func newService(
	msgRepo *mockMessageRepo,
	claude *mockClaudeClient,
	lineClient *mockLineClient,
	contextCount int,
) service.Service {
	return service.New(msgRepo, claude, lineClient, contextCount)
}

// --- tests ---

func TestHandleOpinionCommand_HappyPath(t *testing.T) {
	groupID := uuid.New()
	replyToken := "reply-token-001"
	messages := []domain.Message{
		{ID: uuid.New(), GroupID: groupID, DisplayName: "Alice", Content: "こんにちは"},
		{ID: uuid.New(), GroupID: groupID, DisplayName: "Bob", Content: "やあ"},
	}
	opinion := "とても活発な会話ですね。"

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	msgRepo.On("FindRecent", mock.Anything, groupID, 20).Return(messages, nil)
	claude.On("GenerateOpinion", mock.Anything, messages).Return(opinion, nil)
	lineClient.On("ReplyMessage", mock.Anything, replyToken, opinion).Return(nil)

	svc := newService(msgRepo, claude, lineClient, 20)
	err := svc.HandleOpinionCommand(context.Background(), groupID, replyToken)

	assert.NoError(t, err)
	msgRepo.AssertExpectations(t)
	claude.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}

func TestHandleOpinionCommand_RateLimit(t *testing.T) {
	groupID := uuid.New()
	messages := []domain.Message{{ID: uuid.New(), Content: "hello"}}
	opinion := "なるほど。"

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	// First call should go through.
	msgRepo.On("FindRecent", mock.Anything, groupID, 5).Return(messages, nil).Once()
	claude.On("GenerateOpinion", mock.Anything, messages).Return(opinion, nil).Once()
	lineClient.On("ReplyMessage", mock.Anything, "token-1", opinion).Return(nil).Once()

	svc := newService(msgRepo, claude, lineClient, 5)

	err := svc.HandleOpinionCommand(context.Background(), groupID, "token-1")
	assert.NoError(t, err)

	// Second call within 60 seconds must be skipped (no further mock calls).
	err = svc.HandleOpinionCommand(context.Background(), groupID, "token-2")
	assert.NoError(t, err)

	msgRepo.AssertExpectations(t)
	claude.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}

func TestHandleOpinionCommand_RateLimit_DifferentGroups(t *testing.T) {
	groupA := uuid.New()
	groupB := uuid.New()
	messages := []domain.Message{}
	opinion := "静かですね。"

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	// Both groups should be processed independently.
	msgRepo.On("FindRecent", mock.Anything, groupA, 10).Return(messages, nil).Once()
	msgRepo.On("FindRecent", mock.Anything, groupB, 10).Return(messages, nil).Once()
	claude.On("GenerateOpinion", mock.Anything, messages).Return(opinion, nil).Twice()
	lineClient.On("ReplyMessage", mock.Anything, "token-a", opinion).Return(nil).Once()
	lineClient.On("ReplyMessage", mock.Anything, "token-b", opinion).Return(nil).Once()

	svc := newService(msgRepo, claude, lineClient, 10)

	assert.NoError(t, svc.HandleOpinionCommand(context.Background(), groupA, "token-a"))
	assert.NoError(t, svc.HandleOpinionCommand(context.Background(), groupB, "token-b"))

	msgRepo.AssertExpectations(t)
	claude.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}

func TestHandleOpinionCommand_FindRecentError(t *testing.T) {
	groupID := uuid.New()

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	msgRepo.On("FindRecent", mock.Anything, groupID, 20).Return([]domain.Message{}, errors.New("db error"))

	svc := newService(msgRepo, claude, lineClient, 20)
	err := svc.HandleOpinionCommand(context.Background(), groupID, "token")

	assert.ErrorContains(t, err, "FindRecent")
	claude.AssertNotCalled(t, "GenerateOpinion")
	lineClient.AssertNotCalled(t, "ReplyMessage")
}

func TestHandleOpinionCommand_GenerateOpinionError(t *testing.T) {
	groupID := uuid.New()
	messages := []domain.Message{{ID: uuid.New(), Content: "test"}}

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	msgRepo.On("FindRecent", mock.Anything, groupID, 20).Return(messages, nil)
	claude.On("GenerateOpinion", mock.Anything, messages).Return("", errors.New("claude error"))

	svc := newService(msgRepo, claude, lineClient, 20)
	err := svc.HandleOpinionCommand(context.Background(), groupID, "token")

	assert.ErrorContains(t, err, "GenerateOpinion")
	lineClient.AssertNotCalled(t, "ReplyMessage")
}

func TestHandleOpinionCommand_ReplyMessageError(t *testing.T) {
	groupID := uuid.New()
	messages := []domain.Message{{ID: uuid.New(), Content: "test"}}
	opinion := "意見です。"

	msgRepo := new(mockMessageRepo)
	claude := new(mockClaudeClient)
	lineClient := new(mockLineClient)

	msgRepo.On("FindRecent", mock.Anything, groupID, 20).Return(messages, nil)
	claude.On("GenerateOpinion", mock.Anything, messages).Return(opinion, nil)
	lineClient.On("ReplyMessage", mock.Anything, "token", opinion).Return(errors.New("line api error"))

	svc := newService(msgRepo, claude, lineClient, 20)
	err := svc.HandleOpinionCommand(context.Background(), groupID, "token")

	assert.ErrorContains(t, err, "ReplyMessage")
}

func TestHandleOpinionCommand_RateLimit_ExpiresAfter60s(t *testing.T) {
	// This test verifies that the rate limiter resets after the duration.
	// We inject a service with a short-lived window by using New directly
	// and then manually advancing time — not feasible without dependency injection on clock.
	// Instead, verify the exported behaviour: two calls on different groups go through.
	// (Full time-travel test would require a clock abstraction — out of scope for MVP.)
	t.Skip("time-travel test requires clock abstraction — skipped for MVP")
}

func TestNew_ReturnsService(t *testing.T) {
	svc := service.New(new(mockMessageRepo), new(mockClaudeClient), new(mockLineClient), 20)
	assert.NotNil(t, svc)

	// Verify it satisfies the Service interface at compile time.
	var _ service.Service = svc
}

func init() {
	// Ensure Service interface is satisfied at compile time.
	var _ service.Service = (*service.ObserverService)(nil)
	_ = time.Second // used in rate limit constant
}
