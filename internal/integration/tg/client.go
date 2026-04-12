package tg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/elum-bots/core/internal/mediautil"
)

const defaultBaseURL = "https://api.telegram.org"

type Option func(*Client)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

type Me struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type ChatMember struct {
	UserID int64  `json:"user_id,omitempty"`
	Status string `json:"status"`
}

type File struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

type telegramResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      T      `json:"result"`
}

type setWebhookRequest struct {
	URL            string   `json:"url"`
	SecretToken    string   `json:"secret_token,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

type deleteWebhookRequest struct {
	DropPendingUpdates bool `json:"drop_pending_updates"`
}

type getChatMemberRequest struct {
	ChatID string `json:"chat_id"`
	UserID int64  `json:"user_id"`
}

type getFileRequest struct {
	FileID string `json:"file_id"`
}

func NewClient(token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("tg token is empty")
	}
	c := &Client{
		token:      strings.TrimSpace(token),
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func (c *Client) GetMe(ctx context.Context) (Me, error) {
	var resp telegramResponse[Me]
	if err := c.call(ctx, "getMe", nil, &resp); err != nil {
		return Me{}, err
	}
	return resp.Result, nil
}

func (c *Client) DeleteWebhook(ctx context.Context, dropPendingUpdates bool) error {
	var resp telegramResponse[bool]
	return c.call(ctx, "deleteWebhook", deleteWebhookRequest{DropPendingUpdates: dropPendingUpdates}, &resp)
}

func (c *Client) SetWebhook(ctx context.Context, webhookURL, secretToken string, allowedUpdates []string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return errors.New("tg webhook url is empty")
	}
	var resp telegramResponse[bool]
	return c.call(ctx, "setWebhook", setWebhookRequest{
		URL:            webhookURL,
		SecretToken:    strings.TrimSpace(secretToken),
		AllowedUpdates: allowedUpdates,
	}, &resp)
}

func (c *Client) GetChatMember(ctx context.Context, chatID, userID string) (ChatMember, error) {
	parsedUserID, err := parseUserID(userID)
	if err != nil {
		return ChatMember{}, err
	}
	var resp telegramResponse[ChatMember]
	if err := c.call(ctx, "getChatMember", getChatMemberRequest{
		ChatID: strings.TrimSpace(chatID),
		UserID: parsedUserID,
	}, &resp); err != nil {
		return ChatMember{}, err
	}
	return resp.Result, nil
}

func (m ChatMember) IsMember() bool {
	switch strings.ToLower(strings.TrimSpace(m.Status)) {
	case "creator", "administrator", "member", "restricted":
		return true
	default:
		return false
	}
}

func (c *Client) GetFile(ctx context.Context, fileID string) (File, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return File{}, errors.New("tg file id is empty")
	}
	var resp telegramResponse[File]
	if err := c.call(ctx, "getFile", getFileRequest{FileID: fileID}, &resp); err != nil {
		return File{}, err
	}
	if strings.TrimSpace(resp.Result.FilePath) == "" {
		return File{}, errors.New("tg file path is empty")
	}
	return resp.Result, nil
}

func (c *Client) DownloadFileByID(ctx context.Context, fileID string) ([]byte, string, error) {
	file, err := c.GetFile(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	return c.DownloadFile(ctx, file.FilePath)
}

func (c *Client) DownloadFile(ctx context.Context, filePath string) ([]byte, string, error) {
	if c == nil {
		return nil, "", errors.New("tg client is nil")
	}
	filePath = strings.TrimPrefix(strings.TrimSpace(filePath), "/")
	if filePath == "" {
		return nil, "", errors.New("tg file path is empty")
	}
	rawURL := strings.TrimRight(c.baseURL, "/") + "/file/bot" + c.token + "/" + filePath
	return mediautil.DownloadURL(ctx, c.httpClient, rawURL)
}

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if c == nil {
		return errors.New("tg client is nil")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tg api status: %d body=%s", resp.StatusCode, strings.TrimSpace(string(respRaw)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respRaw, out); err != nil {
		return err
	}
	return ensureTelegramOK(out)
}

func ensureTelegramOK(out any) error {
	value := reflect.ValueOf(out)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return nil
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return nil
	}
	okField := value.FieldByName("OK")
	descriptionField := value.FieldByName("Description")
	if !okField.IsValid() || okField.Kind() != reflect.Bool || !descriptionField.IsValid() || descriptionField.Kind() != reflect.String {
		return nil
	}
	if okField.Bool() {
		return nil
	}
	return fmt.Errorf("tg api error: %s", strings.TrimSpace(descriptionField.String()))
}

func parseUserID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("tg user id is empty")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid tg user id %q", raw)
	}
	return value, nil
}
