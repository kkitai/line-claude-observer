package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/line/line-bot-sdk-go/v8/linebot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kkitai/line-claude-observer/app/internal/domain"
	"github.com/kkitai/line-claude-observer/app/internal/handler"
	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

const testChannelSecret = "test-channel-secret"
const testChannelToken = "test-channel-token"

// --- mock implementations ---

type mockGroupRepo struct{ mock.Mock }

func (m *mockGroupRepo) FindOrCreate(ctx context.Context, lineGroupID, sourceType, name string) (*domain.Group, error) {
	args := m.Called(ctx, lineGroupID, sourceType, name)
	if v := args.Get(0); v != nil {
		return v.(*domain.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockMessageRepo struct{ mock.Mock }

func (m *mockMessageRepo) Save(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageRepo) FindRecent(ctx context.Context, groupID uuid.UUID, limit int) ([]domain.Message, error) {
	args := m.Called(ctx, groupID, limit)
	return args.Get(0).([]domain.Message), args.Error(1)
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

// --- test helpers ---

// lineSignature computes the LINE webhook HMAC-SHA256 signature.
func lineSignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// newTestBot creates a linebot.Client using the test channel secret.
func newTestBot(t *testing.T) *linebot.Client {
	t.Helper()
	bot, err := linebot.New(testChannelSecret, testChannelToken)
	require.NoError(t, err)
	return bot
}

// testGroup returns a dummy domain.Group for testing.
func testGroup() *domain.Group {
	return &domain.Group{
		ID:          uuid.New(),
		LineGroupID: "C_TEST_001",
		SourceType:  "group",
		CreatedAt:   time.Now(),
	}
}

// webhookPayload builds a minimal LINE webhook JSON payload for a text message event.
func webhookPayload(events []map[string]interface{}) string {
	payload := map[string]interface{}{
		"destination": "Udeadbeef",
		"events":      events,
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

// buildRequest creates a signed webhook HTTP request.
func buildRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Line-Signature", lineSignature(testChannelSecret, body))
	return req
}

func newHandler(
	groupRepo repository.GroupRepository,
	msgRepo repository.MessageRepository,
	lineClient *mockLineClient,
	opinionCmd string,
) *handler.WebhookHandler {
	bot, _ := linebot.New(testChannelSecret, testChannelToken)
	return handler.NewWebhookHandler(bot, groupRepo, msgRepo, lineClient, opinionCmd)
}

// --- tests ---

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	h := newHandler(new(mockGroupRepo), new(mockMessageRepo), new(mockLineClient), "/opinion")
	body := `{"destination":"U","events":[]}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Line-Signature", "invalid-signature")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_EmptyEvents(t *testing.T) {
	h := newHandler(new(mockGroupRepo), new(mockMessageRepo), new(mockLineClient), "/opinion")
	body := webhookPayload([]map[string]interface{}{})
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookHandler_TextMessageEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_ALICE").Return("Alice", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.GroupID == group.ID &&
			msg.LineUserID == "U_ALICE" &&
			msg.DisplayName == "Alice" &&
			msg.MessageType == "text" &&
			msg.Content == "Hello!"
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-001",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_ALICE"},
			"timestamp": %s,
			"message": {"type": "text", "id": "M001", "text": "Hello!"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}

func TestWebhookHandler_OpinionCommand(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_BOB").Return("Bob", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.Content == "/opinion" && msg.MessageType == "text"
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-002",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_BOB"},
			"timestamp": %s,
			"message": {"type": "text", "id": "M002", "text": "/opinion"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

func TestWebhookHandler_NonTextMessageEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_CAROL").Return("Carol", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.MessageType == "sticker" && msg.Content == ""
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-003",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_CAROL"},
			"timestamp": %s,
			"message": {"type": "sticker", "id": "M003", "packageId": "1", "stickerId": "2"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}

func TestWebhookHandler_JoinEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_002", "group", "").Return(group, nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "join",
			"source": {"type": "group", "groupId": "C_GROUP_002"},
			"timestamp": %s
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertNotCalled(t, "Save")
}

func TestWebhookHandler_LeaveEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "leave",
			"source": {"type": "group", "groupId": "C_GROUP_003"},
			"timestamp": %s
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertNotCalled(t, "FindOrCreate")
	msgRepo.AssertNotCalled(t, "Save")
}

func TestWebhookHandler_GetDisplayNameFallback(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	// LINE API returns error → should fall back to userID
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_DAVE").Return("", fmt.Errorf("api error"))
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.DisplayName == "U_DAVE" // userID as fallback
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-004",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_DAVE"},
			"timestamp": %s,
			"message": {"type": "text", "id": "M004", "text": "hi"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

func TestWebhookHandler_ImageMessageEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_FRANK").Return("Frank", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.MessageType == "image" && msg.Content == ""
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-img",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_FRANK"},
			"timestamp": %s,
			"message": {"type": "image", "id": "M006"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

func TestWebhookHandler_LocationMessageEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := testGroup()
	groupRepo.On("FindOrCreate", mock.Anything, "C_GROUP_001", "group", "").Return(group, nil)
	lineClient.On("GetGroupMemberProfile", mock.Anything, "C_GROUP_001", "U_GRACE").Return("Grace", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.MessageType == "location" && msg.Content == "Tokyo Station"
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-loc",
			"source": {"type": "group", "groupId": "C_GROUP_001", "userId": "U_GRACE"},
			"timestamp": %s,
			"message": {
				"type": "location",
				"id": "M007",
				"title": "Tokyo",
				"address": "Tokyo Station",
				"latitude": 35.68,
				"longitude": 139.76
			}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

func TestWebhookHandler_UserSourceMessage(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := &domain.Group{ID: uuid.New(), LineGroupID: "U_HENRY", SourceType: "user"}
	groupRepo.On("FindOrCreate", mock.Anything, "U_HENRY", "user", "").Return(group, nil)
	// For user source, getDisplayName returns userID directly (no API call)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.DisplayName == "U_HENRY" && msg.LineUserID == "U_HENRY"
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-usr",
			"source": {"type": "user", "userId": "U_HENRY"},
			"timestamp": %s,
			"message": {"type": "text", "id": "M008", "text": "DM text"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	lineClient.AssertNotCalled(t, "GetGroupMemberProfile")
	lineClient.AssertNotCalled(t, "GetRoomMemberProfile")
}

func TestWebhookHandler_RoomMessageEvent(t *testing.T) {
	groupRepo := new(mockGroupRepo)
	msgRepo := new(mockMessageRepo)
	lineClient := new(mockLineClient)

	group := &domain.Group{ID: uuid.New(), LineGroupID: "R_ROOM_001", SourceType: "room"}
	groupRepo.On("FindOrCreate", mock.Anything, "R_ROOM_001", "room", "").Return(group, nil)
	lineClient.On("GetRoomMemberProfile", mock.Anything, "R_ROOM_001", "U_EVE").Return("Eve", nil)
	msgRepo.On("Save", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
		return msg.DisplayName == "Eve" && msg.GroupID == group.ID
	})).Return(nil)

	h := newHandler(groupRepo, msgRepo, lineClient, "/opinion")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	body := fmt.Sprintf(`{
		"destination": "Udeadbeef",
		"events": [{
			"type": "message",
			"replyToken": "reply-token-005",
			"source": {"type": "room", "roomId": "R_ROOM_001", "userId": "U_EVE"},
			"timestamp": %s,
			"message": {"type": "text", "id": "M005", "text": "room message"}
		}]
	}`, ts)
	req := buildRequest(t, body)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	groupRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	lineClient.AssertExpectations(t)
}
