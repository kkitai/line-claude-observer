package claude

import (
	"context"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/kkitai/line-claude-observer/app/internal/domain"
)

const (
	defaultTimeout = 30 * time.Second
	maxTokens      = 1024
)

// systemPromptText は Claude に渡すシステムプロンプト。
// プロンプトキャッシュの対象とするため CacheControl を付与する。
const systemPromptText = `あなたは第三者として、LINEグループの会話を観察しています。
会話の流れを読み、客観的な立場から意見・分析を述べてください。
回答は日本語で、400文字以内にまとめてください。`

// ClaudeClient は Claude API を使って意見を生成するインターフェース。
type ClaudeClient interface {
	GenerateOpinion(ctx context.Context, messages []domain.Message) (string, error)
}

type claudeClient struct {
	client anthropic.Client
}

// NewClaudeClient は ClaudeClient を生成する。
// テスト時は opts に option.WithBaseURL を渡すことでエンドポイントを差し替えられる。
func NewClaudeClient(apiKey string, opts ...option.RequestOption) ClaudeClient {
	allOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &claudeClient{
		client: anthropic.NewClient(allOpts...),
	}
}

// GenerateOpinion は直近のメッセージを受け取り、Claude で第三者意見を生成する。
// messages は repository.FindRecent の返却値（received_at DESC 順）を想定。
func (c *claudeClient) GenerateOpinion(ctx context.Context, messages []domain.Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	userContent := buildConversationText(messages)

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: maxTokens,
		// システムプロンプトにプロンプトキャッシュを適用する。
		// 同一グループへの連続 /opinion 呼び出しでシステムプロンプト部分のトークンコストを削減できる。
		System: []anthropic.TextBlockParam{
			{
				Text:         systemPromptText,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userContent)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude GenerateOpinion: %w", err)
	}

	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			return t.Text, nil
		}
	}

	return "", fmt.Errorf("claude GenerateOpinion: no text block in response")
}

// buildConversationText は messages（received_at DESC 順）を
// 時系列（古い順）に並べてテキスト化する。
func buildConversationText(messages []domain.Message) string {
	if len(messages) == 0 {
		return "[会話履歴]\n（メッセージなし）\n\n第三者として、この会話についてコメントしてください。"
	}

	var sb strings.Builder
	sb.WriteString("[会話履歴]\n")

	// FindRecent は received_at DESC なので逆順（古い順）に表示
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Content == "" {
			// スタンプ・画像など content が空のメッセージはスキップ
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.LineUserID
		}
		fmt.Fprintf(&sb, "%s: %s\n", name, m.Content)
	}

	sb.WriteString("\n第三者として、この会話についてコメントしてください。")
	return sb.String()
}
