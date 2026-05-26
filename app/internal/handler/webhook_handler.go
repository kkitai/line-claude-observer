package handler

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/line/line-bot-sdk-go/v8/linebot"

	"github.com/kkitai/line-claude-observer/app/internal/domain"
	"github.com/kkitai/line-claude-observer/app/internal/line"
	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

// WebhookHandler handles incoming LINE webhook requests.
type WebhookHandler struct {
	bot         *linebot.Client
	groupRepo   repository.GroupRepository
	messageRepo repository.MessageRepository
	lineClient  line.Client
	opinionCmd  string
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(
	bot *linebot.Client,
	groupRepo repository.GroupRepository,
	messageRepo repository.MessageRepository,
	lineClient line.Client,
	opinionCmd string,
) *WebhookHandler {
	return &WebhookHandler{
		bot:         bot,
		groupRepo:   groupRepo,
		messageRepo: messageRepo,
		lineClient:  lineClient,
		opinionCmd:  opinionCmd,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	events, err := h.bot.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		log.Printf("[webhook] parse error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	for _, event := range events {
		if err := h.handleEvent(r.Context(), event); err != nil {
			log.Printf("[webhook] handle event error (type=%s): %v", event.Type, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleEvent(ctx context.Context, event *linebot.Event) error {
	switch event.Type {
	case linebot.EventTypeMessage:
		return h.handleMessageEvent(ctx, event)
	case linebot.EventTypeJoin:
		return h.handleJoinEvent(ctx, event)
	case linebot.EventTypeLeave:
		log.Printf("[webhook] leave event: source=%+v", event.Source)
		return nil
	default:
		log.Printf("[webhook] unhandled event type: %s", event.Type)
		return nil
	}
}

func (h *WebhookHandler) handleMessageEvent(ctx context.Context, event *linebot.Event) error {
	source := event.Source
	lineGroupID, sourceType := extractGroupInfo(source)

	// グループを取得 or 作成
	group, err := h.groupRepo.FindOrCreate(ctx, lineGroupID, sourceType, "")
	if err != nil {
		return err
	}

	// 表示名を取得（失敗時はuserIDにフォールバック）
	displayName := h.getDisplayName(ctx, source)

	// メッセージ種別とコンテンツを抽出
	msgType, content := extractMessageContent(event.Message)

	// メッセージを保存
	msg := &domain.Message{
		GroupID:     group.ID,
		LineUserID:  source.UserID,
		DisplayName: displayName,
		MessageType: msgType,
		Content:     content,
		ReceivedAt:  event.Timestamp,
	}
	if err := h.messageRepo.Save(ctx, msg); err != nil {
		return err
	}

	// /opinion コマンド検知（Phase 5 で Observer Service を呼び出す）
	if msgType == "text" && strings.HasPrefix(strings.TrimSpace(content), h.opinionCmd) {
		log.Printf("[webhook] opinion command received: groupID=%s replyToken=%s", lineGroupID, event.ReplyToken)
		// TODO: Phase 5 - h.observerService.HandleOpinionCommand(ctx, group.ID, event.ReplyToken)
	}

	return nil
}

func (h *WebhookHandler) handleJoinEvent(ctx context.Context, event *linebot.Event) error {
	source := event.Source
	lineGroupID, sourceType := extractGroupInfo(source)

	_, err := h.groupRepo.FindOrCreate(ctx, lineGroupID, sourceType, "")
	if err != nil {
		return err
	}
	log.Printf("[webhook] joined: lineGroupID=%s sourceType=%s", lineGroupID, sourceType)
	return nil
}

// getDisplayName fetches the member's display name from the LINE API.
// Falls back to userID on error.
func (h *WebhookHandler) getDisplayName(ctx context.Context, source *linebot.EventSource) string {
	if source.UserID == "" {
		return "Unknown"
	}
	switch source.Type {
	case linebot.EventSourceTypeGroup:
		name, err := h.lineClient.GetGroupMemberProfile(ctx, source.GroupID, source.UserID)
		if err != nil {
			log.Printf("[webhook] GetGroupMemberProfile error (userID=%s): %v", source.UserID, err)
			return source.UserID
		}
		return name
	case linebot.EventSourceTypeRoom:
		name, err := h.lineClient.GetRoomMemberProfile(ctx, source.RoomID, source.UserID)
		if err != nil {
			log.Printf("[webhook] GetRoomMemberProfile error (userID=%s): %v", source.UserID, err)
			return source.UserID
		}
		return name
	default:
		return source.UserID
	}
}

// extractGroupInfo returns the line group/room/user ID and source type string.
func extractGroupInfo(source *linebot.EventSource) (lineGroupID, sourceType string) {
	switch source.Type {
	case linebot.EventSourceTypeGroup:
		return source.GroupID, "group"
	case linebot.EventSourceTypeRoom:
		return source.RoomID, "room"
	default:
		return source.UserID, "user"
	}
}

// extractMessageContent returns the message type and text content from a LINE message.
func extractMessageContent(msg linebot.Message) (msgType, content string) {
	switch m := msg.(type) {
	case *linebot.TextMessage:
		return "text", m.Text
	case *linebot.ImageMessage:
		return "image", ""
	case *linebot.VideoMessage:
		return "video", ""
	case *linebot.AudioMessage:
		return "audio", ""
	case *linebot.FileMessage:
		return "file", ""
	case *linebot.StickerMessage:
		return "sticker", ""
	case *linebot.LocationMessage:
		return "location", m.Address
	default:
		if msg != nil {
			return string(msg.Type()), ""
		}
		return "unknown", ""
	}
}
