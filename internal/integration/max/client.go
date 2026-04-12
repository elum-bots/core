package max

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	maxbot "github.com/elum-bots/core/internal/max-bot-api-client-go"
	"github.com/elum-bots/core/internal/mediautil"
)

type Client struct {
	api        *maxbot.Api
	httpClient *http.Client
}

func NewClient(token string) (*Client, error) {
	api, err := maxbot.New(strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	return &Client{
		api:        api,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (c *Client) EnsureWebhookSubscription(ctx context.Context, webhookURL string, updateTypes []string, secret string) error {
	if c == nil || c.api == nil {
		return errors.New("max client is not initialized")
	}
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return errors.New("webhook url is empty")
	}

	subs, err := c.api.Subscriptions.GetSubscriptions(ctx)
	if err != nil {
		return err
	}

	for _, s := range subs.Subscriptions {
		if strings.EqualFold(strings.TrimSpace(s.Url), webhookURL) {
			return nil
		}
	}

	_, err = c.api.Subscriptions.Subscribe(ctx, webhookURL, updateTypes, strings.TrimSpace(secret))
	return err
}

func (c *Client) IsUserMember(ctx context.Context, chatID, userID int64) (bool, error) {
	if c == nil || c.api == nil {
		return false, errors.New("max client is not initialized")
	}
	if chatID == 0 || userID == 0 {
		return false, errors.New("chatID/userID is empty")
	}
	list, err := c.api.Chats.GetSpecificChatMembers(ctx, chatID, []int64{userID})
	if err != nil {
		return false, err
	}
	for _, m := range list.Members {
		if m.UserId == userID {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) DownloadFile(ctx context.Context, rawURL string) ([]byte, string, error) {
	if c == nil {
		return nil, "", errors.New("max client is not initialized")
	}
	return mediautil.DownloadURL(ctx, c.httpClient, rawURL)
}
