package line

import (
	"context"
	"fmt"

	"github.com/line/line-bot-sdk-go/v8/linebot"
)

// Client defines the LINE API operations used by this application.
type Client interface {
	// GetGroupMemberProfile returns the display name of a group member.
	GetGroupMemberProfile(ctx context.Context, groupID, userID string) (string, error)
	// GetRoomMemberProfile returns the display name of a room member.
	GetRoomMemberProfile(ctx context.Context, roomID, userID string) (string, error)
	// ReplyMessage sends a text reply using the given reply token.
	ReplyMessage(ctx context.Context, replyToken, text string) error
}

type apiClient struct {
	bot *linebot.Client
}

// New creates a new LINE API client.
func New(channelSecret, channelAccessToken string, opts ...linebot.ClientOption) (Client, error) {
	bot, err := linebot.New(channelSecret, channelAccessToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("line.New: %w", err)
	}
	return &apiClient{bot: bot}, nil
}

func (c *apiClient) GetGroupMemberProfile(ctx context.Context, groupID, userID string) (string, error) {
	profile, err := c.bot.GetGroupMemberProfile(groupID, userID).WithContext(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("GetGroupMemberProfile: %w", err)
	}
	return profile.DisplayName, nil
}

func (c *apiClient) GetRoomMemberProfile(ctx context.Context, roomID, userID string) (string, error) {
	profile, err := c.bot.GetRoomMemberProfile(roomID, userID).WithContext(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("GetRoomMemberProfile: %w", err)
	}
	return profile.DisplayName, nil
}

func (c *apiClient) ReplyMessage(ctx context.Context, replyToken, text string) error {
	_, err := c.bot.ReplyMessage(replyToken, linebot.NewTextMessage(text)).WithContext(ctx).Do()
	if err != nil {
		return fmt.Errorf("ReplyMessage: %w", err)
	}
	return nil
}
