package claude_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkitai/line-claude-observer/app/internal/claude"
	"github.com/kkitai/line-claude-observer/app/internal/domain"
)

// mockMessagesResponse は /v1/messages に対してモックレスポンスを返すハンドラーを生成する。
func mockMessagesResponse(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(nil, "/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_test_01",
			"type": "message",
			"role": "assistant",
			"content": []map[string]string{
				{"type": "text", "text": text},
			},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":                100,
				"output_tokens":               50,
				"cache_creation_input_tokens": 200,
				"cache_read_input_tokens":     0,
			},
		})
	}
}

// newTestClient は httptest.Server を使ったテスト用 ClaudeClient を生成する。
func newTestClient(t *testing.T, handler http.Handler) claude.ClaudeClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return claude.NewClaudeClient(
		"test-api-key",
		option.WithBaseURL(server.URL+"/"),
		option.WithMaxRetries(0),
	)
}

// sampleMessages は時系列（DESC 順）のサンプルメッセージを返す。
func sampleMessages() []domain.Message {
	now := time.Now()
	return []domain.Message{
		{
			ID:          uuid.New(),
			LineUserID:  "U002",
			DisplayName: "Bob",
			MessageType: "text",
			Content:     "ラーメンがいい",
			ReceivedAt:  now.Add(-1 * time.Minute), // 新しい（DESC 順で先頭）
		},
		{
			ID:          uuid.New(),
			LineUserID:  "U001",
			DisplayName: "Alice",
			MessageType: "text",
			Content:     "今日のランチどうする？",
			ReceivedAt:  now.Add(-2 * time.Minute),
		},
	}
}

func TestGenerateOpinion(t *testing.T) {
	t.Run("意見テキストを返す", func(t *testing.T) {
		expected := "二人がランチについて話し合っています。良好なコミュニケーションです。"
		client := newTestClient(t, mockMessagesResponse(expected))

		opinion, err := client.GenerateOpinion(context.Background(), sampleMessages())
		require.NoError(t, err)
		assert.Equal(t, expected, opinion)
	})

	t.Run("APIエラー時はエラーを返す", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w,
				`{"type":"error","error":{"type":"invalid_request_error","message":"Bad request"}}`,
				http.StatusBadRequest,
			)
		})
		client := newTestClient(t, handler)

		_, err := client.GenerateOpinion(context.Background(), sampleMessages())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "claude GenerateOpinion")
	})

	t.Run("メッセージが空でもAPIを呼び出す", func(t *testing.T) {
		expected := "会話履歴がありません。"
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			// リクエストボディに「メッセージなし」テキストが含まれることを確認
			var reqBody map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
			msgs := reqBody["messages"].([]any)
			firstMsg := msgs[0].(map[string]any)
			content := firstMsg["content"].([]any)
			firstBlock := content[0].(map[string]any)
			assert.Contains(t, firstBlock["text"], "メッセージなし")

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_test_02",
				"type": "message",
				"role": "assistant",
				"content": []map[string]string{
					{"type": "text", "text": expected},
				},
				"model":       "claude-sonnet-4-6",
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
			})
		})
		client := newTestClient(t, handler)

		opinion, err := client.GenerateOpinion(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, expected, opinion)
		assert.True(t, called)
	})

	t.Run("content空のメッセージ（スタンプ等）はスキップされる", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
			msgs := reqBody["messages"].([]any)
			firstMsg := msgs[0].(map[string]any)
			content := firstMsg["content"].([]any)
			firstBlock := content[0].(map[string]any)
			// スタンプのみのメッセージはテキスト化されないことを確認
			assert.NotContains(t, firstBlock["text"], "U001:")

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_test_03",
				"type": "message",
				"role": "assistant",
				"content": []map[string]string{
					{"type": "text", "text": "スタンプが送られました。"},
				},
				"model":       "claude-sonnet-4-6",
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
			})
		})
		client := newTestClient(t, handler)

		messages := []domain.Message{
			{
				ID:          uuid.New(),
				LineUserID:  "U001",
				DisplayName: "Alice",
				MessageType: "sticker",
				Content:     "", // スタンプは content が空
				ReceivedAt:  time.Now(),
			},
		}

		opinion, err := client.GenerateOpinion(context.Background(), messages)
		require.NoError(t, err)
		assert.Equal(t, "スタンプが送られました。", opinion)
	})

	t.Run("メッセージは古い順（時系列）で並べられる", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
			msgs := reqBody["messages"].([]any)
			firstMsg := msgs[0].(map[string]any)
			content := firstMsg["content"].([]any)
			firstBlock := content[0].(map[string]any)
			text := firstBlock["text"].(string)
			// sampleMessages() は DESC 順なので、古い "Alice: 今日のランチ" が先に来るはず
			aliceIdx := indexOf(text, "Alice:")
			bobIdx := indexOf(text, "Bob:")
			assert.Less(t, aliceIdx, bobIdx, "Alice のメッセージが Bob より先に表示されるべき")

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_test_04",
				"type": "message",
				"role": "assistant",
				"content": []map[string]string{
					{"type": "text", "text": "会話の順番が正しいです。"},
				},
				"model":       "claude-sonnet-4-6",
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 20, "output_tokens": 10},
			})
		})
		client := newTestClient(t, handler)

		_, err := client.GenerateOpinion(context.Background(), sampleMessages())
		require.NoError(t, err)
	})
}

// indexOf は文字列 s 中の substr の開始位置を返す（なければ -1）。
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
