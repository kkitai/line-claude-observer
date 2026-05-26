package line_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/line/line-bot-sdk-go/v8/linebot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkitai/line-claude-observer/app/internal/line"
)

// newTestClient creates a Client backed by an httptest server.
func newTestClient(t *testing.T, handler http.Handler) (line.Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := line.New(
		"test-secret",
		"test-token",
		linebot.WithEndpointBase(server.URL+"/"),
	)
	require.NoError(t, err)
	return client, server
}

func TestNew(t *testing.T) {
	t.Run("creates client with valid credentials", func(t *testing.T) {
		client, err := line.New("secret", "token")
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestGetGroupMemberProfile(t *testing.T) {
	t.Run("returns display name on success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v2/bot/group/G001/member/U001", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"userId":        "U001",
				"displayName":   "Alice",
				"pictureUrl":    "",
				"statusMessage": "",
			})
		})

		client, _ := newTestClient(t, handler)
		name, err := client.GetGroupMemberProfile(context.Background(), "G001", "U001")
		require.NoError(t, err)
		assert.Equal(t, "Alice", name)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not found"}`, http.StatusNotFound)
		})

		client, _ := newTestClient(t, handler)
		_, err := client.GetGroupMemberProfile(context.Background(), "G999", "U999")
		assert.Error(t, err)
	})
}

func TestGetRoomMemberProfile(t *testing.T) {
	t.Run("returns display name on success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v2/bot/room/R001/member/U001", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"userId":      "U001",
				"displayName": "Bob",
			})
		})

		client, _ := newTestClient(t, handler)
		name, err := client.GetRoomMemberProfile(context.Background(), "R001", "U001")
		require.NoError(t, err)
		assert.Equal(t, "Bob", name)
	})
}

func TestReplyMessage(t *testing.T) {
	t.Run("sends reply successfully", func(t *testing.T) {
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v2/bot/message/reply", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "reply-token-123", body["replyToken"])

			called = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		})

		client, _ := newTestClient(t, handler)
		err := client.ReplyMessage(context.Background(), "reply-token-123", "Hello!")
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
		})

		client, _ := newTestClient(t, handler)
		err := client.ReplyMessage(context.Background(), "token", "text")
		assert.Error(t, err)
	})
}
