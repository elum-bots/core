package tg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elum-bots/core/internal/apiflow"
	"github.com/elum-bots/core/internal/bot"
)

const defaultBaseURL = "https://api.telegram.org"

var anchorOpenTagRE = regexp.MustCompile(`(?i)<a\s+href="[^"]+">`)

type Option func(*Adapter)

type Adapter struct {
	token       string
	baseURL     string
	client      *http.Client
	pollTimeout int
	pollLimit   int
	sendTimeout time.Duration
	flow        *apiflow.Dispatcher

	mu     sync.Mutex
	offset int64
}

func NewAdapter(token string, opts ...Option) *Adapter {
	a := &Adapter{
		token:       token,
		baseURL:     defaultBaseURL,
		client:      &http.Client{},
		pollTimeout: 30,
		pollLimit:   100,
		sendTimeout: 3 * time.Minute,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithBaseURL(baseURL string) Option {
	return func(a *Adapter) {
		if baseURL != "" {
			a.baseURL = strings.TrimSuffix(baseURL, "/")
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(a *Adapter) {
		if client != nil {
			a.client = client
		}
	}
}

func WithPolling(timeoutSec, limit int) Option {
	return func(a *Adapter) {
		if timeoutSec > 0 {
			a.pollTimeout = timeoutSec
		}
		if limit > 0 {
			a.pollLimit = limit
		}
	}
}

func WithSendTimeout(timeout time.Duration) Option {
	return func(a *Adapter) {
		if timeout > 0 {
			a.sendTimeout = timeout
		}
	}
}

func WithAPIDispatcher(d *apiflow.Dispatcher) Option {
	return func(a *Adapter) {
		a.flow = d
	}
}

func (a *Adapter) Name() string {
	return "tg"
}

func (a *Adapter) Start(ctx context.Context, dispatch bot.Dispatcher) error {
	if a.token == "" {
		return bot.ErrInvalidInput
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for {
		select {
		case <-runCtx.Done():
			wg.Wait()
			if err := firstErr(errCh); err != nil {
				return err
			}
			return runCtx.Err()
		default:
		}

		updates, err := a.getUpdates(runCtx)
		if err != nil {
			if ferr := firstErr(errCh); ferr != nil {
				wg.Wait()
				return ferr
			}
			wg.Wait()
			return err
		}

		for _, u := range updates {
			norm, ok := normalizeUpdate(u)
			if !ok || norm.ChatID == "" || norm.UserID == "" {
				continue
			}
			wg.Add(1)
			go func(up bot.Update) {
				defer wg.Done()
				if err := dispatch.Dispatch(runCtx, up); err != nil && err != bot.ErrUnknownEvent {
					select {
					case errCh <- err:
					default:
					}
					cancel()
				}
			}(norm)
		}

		if err := firstErr(errCh); err != nil {
			wg.Wait()
			return err
		}
	}
}

func (a *Adapter) Stop(context.Context) error {
	return nil
}

func (a *Adapter) Send(ctx context.Context, chatID string, msg bot.OutMessage) error {
	if a.token == "" {
		return bot.ErrInvalidInput
	}
	if len(msg.Media) > 0 {
		return a.sendMedia(ctx, chatID, msg)
	}

	req := sendMessageRequest{Text: msg.Text}
	if n, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		req.ChatID = n
	} else {
		req.ChatID = chatID
	}

	if len(msg.Buttons) > 0 {
		req.ReplyMarkup = buildInlineKeyboard(msg.Buttons)
	}
	if needsHTMLParse(req.Text) {
		req.Text = sanitizeHTMLText(req.Text)
		req.ParseMode = "HTML"
	}

	sctx, cancel := context.WithTimeout(apiflow.WithClass(ctx, apiflow.ClassResponse), a.sendTimeout)
	defer cancel()
	return a.call(sctx, "sendMessage", req, nil)
}

func (a *Adapter) sendMedia(ctx context.Context, chatID string, msg bot.OutMessage) error {
	if len(msg.Media) == 1 {
		replyMarkup := (*inlineKeyboard)(nil)
		if len(msg.Buttons) > 0 {
			replyMarkup = buildInlineKeyboard(msg.Buttons)
		}
		caption := msg.Media[0].Caption
		if caption == "" {
			caption = msg.Text
		}
		return a.sendSingleMedia(ctx, chatID, msg.Media[0], caption, replyMarkup)
	}
	if err := a.sendMediaGroup(ctx, chatID, msg); err != nil {
		return err
	}
	if len(msg.Buttons) == 0 {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = "Выберите действие"
	}
	return a.sendTextWithButtons(ctx, chatID, text, msg.Buttons)
}

func (a *Adapter) sendTextWithButtons(ctx context.Context, chatID string, text string, buttons []bot.ButtonRow) error {
	req := sendMessageRequest{Text: text}
	if n, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		req.ChatID = n
	} else {
		req.ChatID = chatID
	}
	req.ReplyMarkup = buildInlineKeyboard(buttons)
	if needsHTMLParse(req.Text) {
		req.Text = sanitizeHTMLText(req.Text)
		req.ParseMode = "HTML"
	}
	sctx, cancel := context.WithTimeout(apiflow.WithClass(ctx, apiflow.ClassResponse), a.sendTimeout)
	defer cancel()
	return a.call(sctx, "sendMessage", req, nil)
}

func (a *Adapter) sendSingleMedia(ctx context.Context, chatID string, media bot.OutMedia, caption string, replyMarkup *inlineKeyboard) error {
	if err := a.acquire(ctx, apiflow.ClassResponse); err != nil {
		return err
	}

	method := ""
	field := ""
	switch media.Kind {
	case bot.OutMediaImage:
		method = "sendPhoto"
		field = "photo"
	case bot.OutMediaVideo:
		method = "sendVideo"
		field = "video"
	case bot.OutMediaAudio:
		method = "sendAudio"
		field = "audio"
	case bot.OutMediaFile:
		method = "sendDocument"
		field = "document"
	default:
		return bot.ErrInvalidInput
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("chat_id", chatID)
	if caption != "" {
		_ = w.WriteField("caption", caption)
	}
	if replyMarkup != nil {
		b, err := json.Marshal(replyMarkup)
		if err != nil {
			return err
		}
		_ = w.WriteField("reply_markup", string(b))
	}
	if len(media.Bytes) > 0 {
		part, err := w.CreateFormFile(field, mediaFileName(media))
		if err != nil {
			return err
		}
		if _, err := part.Write(media.Bytes); err != nil {
			return err
		}
	} else {
		f, err := os.Open(media.Path)
		if err != nil {
			return a.sendSingleMediaByRef(ctx, chatID, method, field, media.Path, caption, replyMarkup)
		}
		defer f.Close()

		part, err := w.CreateFormFile(field, mediaFileName(media))
		if err != nil {
			return err
		}
		if _, err := io.Copy(part, f); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/%s", strings.TrimSuffix(a.baseURL, "/"), a.token, method)
	req, err := http.NewRequestWithContext(apiflow.WithClass(ctx, apiflow.ClassResponse), http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	sctx, cancel := context.WithTimeout(ctx, a.sendTimeout)
	defer cancel()
	req = req.WithContext(sctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tg api status: %d", resp.StatusCode)
	}
	var ack struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return err
	}
	if !ack.OK {
		return fmt.Errorf("tg api error")
	}
	return nil
}

func (a *Adapter) sendSingleMediaByRef(ctx context.Context, chatID, method, field, ref, caption string, replyMarkup *inlineKeyboard) error {
	req := map[string]any{
		"chat_id": chatID,
		field:     ref,
	}
	if caption != "" {
		req["caption"] = caption
	}
	if replyMarkup != nil {
		req["reply_markup"] = replyMarkup
	}
	sctx, cancel := context.WithTimeout(ctx, a.sendTimeout)
	defer cancel()
	return a.call(sctx, method, req, nil)
}

func buildInlineKeyboard(rows []bot.ButtonRow) *inlineKeyboard {
	keyboard := make([][]inlineButton, 0, len(rows))
	for _, row := range rows {
		items := make([]inlineButton, 0, len(row))
		for _, btn := range row {
			item := inlineButton{Text: btn.Text}
			if btn.Kind == bot.ButtonKindURL {
				item.URL = btn.URL
			} else {
				item.CallbackData = btn.ID
			}
			items = append(items, item)
		}
		keyboard = append(keyboard, items)
	}
	return &inlineKeyboard{InlineKeyboard: keyboard}
}

func (a *Adapter) sendMediaGroup(ctx context.Context, chatID string, msg bot.OutMessage) error {
	if err := a.acquire(ctx, apiflow.ClassResponse); err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("chat_id", chatID)

	media := make([]map[string]any, 0, len(msg.Media))

	for i, item := range msg.Media {
		kind, _, err := mediaKindToTG(item.Kind)
		if err != nil {
			return err
		}
		fileField := "file" + strconv.Itoa(i)
		part, err := w.CreateFormFile(fileField, mediaFileName(item))
		if err != nil {
			return err
		}
		if len(item.Bytes) > 0 {
			if _, err := part.Write(item.Bytes); err != nil {
				return err
			}
		} else {
			file, err := os.Open(item.Path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, file); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}

		entry := map[string]any{
			"type":  kind,
			"media": "attach://" + fileField,
		}
		if i == 0 && len(msg.Buttons) == 0 {
			caption := item.Caption
			if caption == "" {
				caption = msg.Text
			}
			if caption != "" {
				entry["caption"] = caption
			}
		}
		media = append(media, entry)
	}
	mediaJSON, err := json.Marshal(media)
	if err != nil {
		return err
	}
	_ = w.WriteField("media", string(mediaJSON))
	if err := w.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/sendMediaGroup", strings.TrimSuffix(a.baseURL, "/"), a.token)
	req, err := http.NewRequestWithContext(apiflow.WithClass(ctx, apiflow.ClassResponse), http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	sctx, cancel := context.WithTimeout(ctx, a.sendTimeout)
	defer cancel()
	req = req.WithContext(sctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tg api status: %d", resp.StatusCode)
	}
	var ack struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return err
	}
	if !ack.OK {
		return fmt.Errorf("tg api error")
	}
	return nil
}

func (a *Adapter) acquire(ctx context.Context, class apiflow.Class) error {
	flow := a.flow
	if flow == nil {
		flow = apiflow.TG()
	}
	if flow == nil {
		return nil
	}
	return flow.Acquire(apiflow.WithClass(ctx, class), class)
}

func mediaKindToTG(kind bot.OutMediaKind) (typ string, field string, err error) {
	switch kind {
	case bot.OutMediaImage:
		return "photo", "photo", nil
	case bot.OutMediaVideo:
		return "video", "video", nil
	case bot.OutMediaAudio:
		return "audio", "audio", nil
	default:
		return "", "", bot.ErrInvalidInput
	}
}

func mediaFileName(media bot.OutMedia) string {
	name := strings.TrimSpace(media.Name)
	if name != "" {
		return name
	}
	if strings.TrimSpace(media.Path) != "" {
		return media.Path
	}
	switch media.Kind {
	case bot.OutMediaImage:
		return "image.jpg"
	case bot.OutMediaVideo:
		return "video.mp4"
	case bot.OutMediaAudio:
		return "audio.mp3"
	case bot.OutMediaFile:
		return "file.bin"
	default:
		return "media.bin"
	}
}

func (a *Adapter) getUpdates(ctx context.Context) ([]tgUpdate, error) {
	a.mu.Lock()
	offset := a.offset
	a.mu.Unlock()

	req := getUpdatesRequest{Offset: offset, Timeout: a.pollTimeout, Limit: a.pollLimit}
	var resp tgResponse[tgUpdate]
	pollCtx, cancel := context.WithTimeout(apiflow.WithClass(ctx, apiflow.ClassSubscription), time.Duration(a.pollTimeout+15)*time.Second)
	defer cancel()
	if err := a.call(pollCtx, "getUpdates", req, &resp); err != nil {
		return nil, err
	}

	maxID := offset
	for _, u := range resp.Result {
		if u.UpdateID >= maxID {
			maxID = u.UpdateID + 1
		}
	}
	a.mu.Lock()
	a.offset = maxID
	a.mu.Unlock()
	return resp.Result, nil
}

func (a *Adapter) call(ctx context.Context, method string, payload any, out any) error {
	if err := a.acquire(ctx, apiflow.ClassFromContext(ctx)); err != nil {
		return err
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/%s", strings.TrimSuffix(a.baseURL, "/"), a.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tg api status: %d", resp.StatusCode)
	}

	if out == nil {
		var ack struct {
			OK bool `json:"ok"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
			return err
		}
		if !ack.OK {
			return fmt.Errorf("tg api error")
		}
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}

	if r, ok := out.(*tgResponse[tgUpdate]); ok && !r.OK {
		return fmt.Errorf("tg api error")
	}
	return nil
}

func firstErr(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func needsHTMLParse(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "<a href=") && strings.Contains(t, "</a>")
}

func sanitizeHTMLText(text string) string {
	if !needsHTMLParse(text) {
		return text
	}

	placeholders := make(map[string]string)
	index := 0

	text = anchorOpenTagRE.ReplaceAllStringFunc(text, func(tag string) string {
		key := fmt.Sprintf("__TG_HTML_OPEN_%d__", index)
		index++
		placeholders[key] = tag
		return key
	})

	const closeTagPlaceholder = "__TG_HTML_CLOSE__"
	text = strings.ReplaceAll(text, "</a>", closeTagPlaceholder)

	text = html.EscapeString(text)

	for key, tag := range placeholders {
		text = strings.ReplaceAll(text, key, tag)
	}
	text = strings.ReplaceAll(text, closeTagPlaceholder, "</a>")

	return text
}

func normalizeUpdate(u tgUpdate) (bot.Update, bool) {
	out := bot.Update{Platform: "tg", Payload: map[string]any{}, Raw: u}
	out.Message.Kind = bot.MessageKindOther

	if u.CallbackQuery != nil {
		cq := u.CallbackQuery
		if cq.Message == nil || !isPrivateChat(cq.Message.Chat) {
			return bot.Update{}, false
		}
		out.ButtonID = cq.Data
		if cq.From != nil {
			out.UserID = strconv.FormatInt(cq.From.ID, 10)
		}
		if cq.Message != nil {
			out.ChatID = strconv.FormatInt(cq.Message.Chat.ID, 10)
			out.MessageID = strconv.FormatInt(cq.Message.MessageID, 10)
		}
		return out, true
	}

	if u.Message == nil {
		return bot.Update{}, false
	}

	m := u.Message
	if !isPrivateChat(m.Chat) {
		return bot.Update{}, false
	}
	out.ChatID = strconv.FormatInt(m.Chat.ID, 10)
	out.MessageID = strconv.FormatInt(m.MessageID, 10)
	if m.From != nil {
		out.UserID = strconv.FormatInt(m.From.ID, 10)
	}

	if m.Text != "" {
		out.Message.Kind = bot.MessageKindText
		out.Message.Text = m.Text
		return out, true
	}
	if len(m.Photo) > 0 {
		p := m.Photo[len(m.Photo)-1]
		out.Message.Kind = bot.MessageKindPhoto
		out.Message.Text = m.Caption
		out.Message.Attachments = []bot.Attachment{{ID: p.FileID, MIME: "image/*", Size: int64(p.FileSize)}}
		return out, true
	}
	if m.Video != nil {
		out.Message.Kind = bot.MessageKindVideo
		out.Message.Text = m.Caption
		out.Message.Attachments = []bot.Attachment{{ID: m.Video.FileID, MIME: "video/*", Size: int64(m.Video.FileSize)}}
		return out, true
	}
	if m.Audio != nil {
		out.Message.Kind = bot.MessageKindAudio
		out.Message.Text = m.Caption
		out.Message.Attachments = []bot.Attachment{{ID: m.Audio.FileID, MIME: "audio/*", Size: int64(m.Audio.FileSize)}}
		return out, true
	}
	if m.Voice != nil {
		out.Message.Kind = bot.MessageKindVoice
		out.Message.Attachments = []bot.Attachment{{ID: m.Voice.FileID, MIME: "audio/ogg", Size: int64(m.Voice.FileSize)}}
		return out, true
	}
	if m.Document != nil {
		mime := m.Document.MimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		out.Message.Kind = bot.MessageKindFile
		out.Message.Text = m.Caption
		out.Message.Attachments = []bot.Attachment{{ID: m.Document.FileID, Name: m.Document.FileName, MIME: mime, Size: int64(m.Document.FileSize)}}
	}
	return out, true
}

func isPrivateChat(chat tgChat) bool {
	return strings.EqualFold(strings.TrimSpace(chat.Type), "private")
}

type getUpdatesRequest struct {
	Offset  int64 `json:"offset,omitempty"`
	Timeout int   `json:"timeout,omitempty"`
	Limit   int   `json:"limit,omitempty"`
}

type sendMessageRequest struct {
	ChatID      any             `json:"chat_id"`
	Text        string          `json:"text"`
	ParseMode   string          `json:"parse_mode,omitempty"`
	ReplyMarkup *inlineKeyboard `json:"reply_markup,omitempty"`
}

type inlineKeyboard struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type tgResponse[T any] struct {
	OK     bool `json:"ok"`
	Result []T  `json:"result"`
}

type tgUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *tgMessage       `json:"message,omitempty"`
	CallbackQuery *tgCallbackQuery `json:"callback_query,omitempty"`
}

type tgCallbackQuery struct {
	ID      string     `json:"id"`
	From    *tgUser    `json:"from,omitempty"`
	Data    string     `json:"data,omitempty"`
	Message *tgMessage `json:"message,omitempty"`
}

type tgMessage struct {
	MessageID int64       `json:"message_id"`
	From      *tgUser     `json:"from,omitempty"`
	Chat      tgChat      `json:"chat"`
	Text      string      `json:"text,omitempty"`
	Caption   string      `json:"caption,omitempty"`
	Photo     []tgPhoto   `json:"photo,omitempty"`
	Video     *tgFile     `json:"video,omitempty"`
	Audio     *tgFile     `json:"audio,omitempty"`
	Voice     *tgFile     `json:"voice,omitempty"`
	Document  *tgDocument `json:"document,omitempty"`
}

type tgPhoto struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size,omitempty"`
}

type tgFile struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size,omitempty"`
}

type tgDocument struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	FileSize int    `json:"file_size,omitempty"`
}

type tgUser struct {
	ID int64 `json:"id"`
}

type tgChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type,omitempty"`
}

var _ bot.Adapter = (*Adapter)(nil)
