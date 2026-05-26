package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kkitai/line-claude-observer/app/internal/claude"
	"github.com/kkitai/line-claude-observer/app/internal/line"
	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

const rateLimitDuration = 60 * time.Second

// Service is the interface used by the webhook handler.
type Service interface {
	HandleOpinionCommand(ctx context.Context, groupID uuid.UUID, replyToken string) error
}

// ObserverService fetches recent messages, generates a Claude opinion, and replies via LINE.
type ObserverService struct {
	messageRepo  repository.MessageRepository
	claudeClient claude.ClaudeClient
	lineClient   line.Client
	contextCount int
	mu           sync.Mutex
	lastOpinion  map[uuid.UUID]time.Time
}

// New creates a new ObserverService.
func New(
	messageRepo repository.MessageRepository,
	claudeClient claude.ClaudeClient,
	lineClient line.Client,
	contextCount int,
) *ObserverService {
	return &ObserverService{
		messageRepo:  messageRepo,
		claudeClient: claudeClient,
		lineClient:   lineClient,
		contextCount: contextCount,
		lastOpinion:  make(map[uuid.UUID]time.Time),
	}
}

// HandleOpinionCommand fetches recent messages, generates an opinion, and sends a LINE reply.
// Calls within 60 seconds for the same group are silently skipped.
func (s *ObserverService) HandleOpinionCommand(ctx context.Context, groupID uuid.UUID, replyToken string) error {
	if s.isRateLimited(groupID) {
		log.Printf("[service] opinion command rate-limited: groupID=%s", groupID)
		return nil
	}

	messages, err := s.messageRepo.FindRecent(ctx, groupID, s.contextCount)
	if err != nil {
		return fmt.Errorf("HandleOpinionCommand FindRecent: %w", err)
	}

	opinion, err := s.claudeClient.GenerateOpinion(ctx, messages)
	if err != nil {
		return fmt.Errorf("HandleOpinionCommand GenerateOpinion: %w", err)
	}

	if err := s.lineClient.ReplyMessage(ctx, replyToken, opinion); err != nil {
		return fmt.Errorf("HandleOpinionCommand ReplyMessage: %w", err)
	}

	return nil
}

// isRateLimited returns true if the group has sent /opinion within the last 60 seconds.
// Records the current call time when returning false.
func (s *ObserverService) isRateLimited(groupID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	last, ok := s.lastOpinion[groupID]
	if ok && time.Since(last) < rateLimitDuration {
		return true
	}
	s.lastOpinion[groupID] = time.Now()
	return false
}
