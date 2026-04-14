package max

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/elum-bots/core/internal/apiflow"
	"github.com/elum-bots/core/internal/bot"
	maxintegration "github.com/elum-bots/core/internal/integration/max"
	maxbot "github.com/elum-bots/core/internal/max-bot-api-client-go"
	"github.com/elum-bots/core/internal/max-bot-api-client-go/schemes"
)

type Option func(*Adapter)

type Adapter struct {
	api            *maxbot.Api
	updates        chan schemes.UpdateInterface
	webhookHandler http.HandlerFunc
	flow           *apiflow.Dispatcher
	httpTimeout    time.Duration
}

type recipientIDs struct {
	chat       int64
	user       int64
	chatUser   int64
	preferUser bool
}

type recipientRoute struct {
	name string
	chat int64
	user int64
}

func NewAdapter(token string, opts ...Option) (*Adapter, error) {
	api, err := maxbot.New(strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		api:     api,
		updates: make(chan schemes.UpdateInterface, 1024),
	}
	a.httpTimeout = 30 * time.Second
	a.webhookHandler = api.GetHandler(a.updates)
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}

func WithAPIDispatcher(d *apiflow.Dispatcher) Option {
	return func(a *Adapter) {
		a.flow = d
	}
}

func WithHTTPTimeout(timeout time.Duration) Option {
	return func(a *Adapter) {
		if a == nil || a.api == nil || timeout <= 0 {
			return
		}
		a.api.SetHTTPTimeout(timeout)
		a.httpTimeout = timeout
	}
}

func (a *Adapter) Name() string {
	return "max"
}

func (a *Adapter) Start(ctx context.Context, dispatch bot.Dispatcher) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case u := <-a.updates:
			upd, ok := normalizeUpdate(u)
			if !ok {
				log.Printf("max update skipped: type=%T", u)
				continue
			}
			if err := dispatch.Dispatch(ctx, upd); err != nil && err != bot.ErrUnknownEvent {
				log.Printf(
					"max update dispatch error: type=%T user=%s chat=%s button=%s err=%s",
					u,
					upd.UserID,
					upd.ChatID,
					upd.ButtonID,
					describeSendErr(err),
				)
				continue
			}
		}
	}
}

func (a *Adapter) Stop(context.Context) error {
	return nil
}

func (a *Adapter) Send(ctx context.Context, chatID string, msg bot.OutMessage) error {
	target, err := resolveRecipients(ctx, chatID)
	if err != nil {
		return err
	}
	if len(msg.Media) == 0 {
		return a.sendText(ctx, target, msg.Text, msg.Buttons)
	}
	if len(msg.Media) == 1 {
		caption := strings.TrimSpace(msg.Media[0].Caption)
		if caption == "" {
			caption = msg.Text
		}
		return a.sendSingleMedia(ctx, target, msg.Media[0], caption, msg.Buttons)
	}
	for i, media := range msg.Media {
		caption := media.Caption
		if i == 0 && strings.TrimSpace(caption) == "" {
			caption = msg.Text
		}
		if err := a.sendSingleMedia(ctx, target, media, caption, nil); err != nil {
			return err
		}
	}
	if len(msg.Buttons) > 0 {
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			text = "Выберите действие"
		}
		return a.sendText(ctx, target, text, msg.Buttons)
	}
	return nil
}

func (a *Adapter) HandleWebhook(body []byte) error {
	if a.webhookHandler == nil {
		return fmt.Errorf("max webhook handler is not initialized")
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	a.webhookHandler(rec, req)
	if rec.Code >= 200 && rec.Code < 300 {
		return nil
	}
	return fmt.Errorf(
		"max webhook parse failed: status=%d body=%s request=%s",
		rec.Code,
		strings.TrimSpace(rec.Body.String()),
		trimForLog(string(body), 400),
	)
}

func (a *Adapter) sendText(ctx context.Context, target recipientIDs, text string, buttons []bot.ButtonRow) error {
	routes := candidateRoutes(target)
	var lastErr error
	for _, route := range routes {
		msg := maxbot.NewMessage().SetText(text)
		applyRoute(msg, route)
		applyTextFormat(msg, text)
		applyButtons(msg, buttons)
		if err := a.sendMAX(ctx, msg); err != nil {
			lastErr = err
			log.Printf(
				"max send failed route=%s recipient=%s kind=text text=%q buttons=%d err=%s",
				route.name,
				routeRecipient(route),
				trimForLog(text, 120),
				len(buttons),
				describeSendErr(err),
			)
			if !isRecipientRouteErr(err) {
				if answered, aerr := a.answerCallbackFallback(ctx, text, buttons, err); answered {
					return aerr
				}
				return err
			}
			continue
		}
		return nil
	}
	if answered, aerr := a.answerCallbackFallback(ctx, text, buttons, lastErr); answered {
		return aerr
	}
	return lastErr
}

func (a *Adapter) sendSingleMedia(ctx context.Context, target recipientIDs, media bot.OutMedia, caption string, buttons []bot.ButtonRow) error {
	build := func(route recipientRoute) (*maxbot.Message, error) {
		msg := maxbot.NewMessage().SetText(caption)
		applyTextFormat(msg, caption)
		applyRoute(msg, route)
		switch media.Kind {
		case bot.OutMediaImage:
			if len(media.Bytes) > 0 {
				photo, err := a.api.Uploads.UploadPhotoFromReaderWithName(apiflow.WithClass(ctx, apiflow.ClassResponse), bytes.NewReader(media.Bytes), mediaFileName(media))
				if err != nil {
					return nil, err
				}
				msg.AddPhoto(photo)
			} else if looksLikeStoredMediaRef(media.Path) {
				if !maxintegration.AttachPhotoReference(msg, media.Path) {
					return nil, fmt.Errorf("failed to attach max photo reference")
				}
			} else {
				photo, err := a.api.Uploads.UploadPhotoFromFile(apiflow.WithClass(ctx, apiflow.ClassResponse), media.Path)
				if err != nil {
					return nil, err
				}
				msg.AddPhoto(photo)
			}
		case bot.OutMediaVideo:
			if len(media.Bytes) > 0 {
				video, err := a.api.Uploads.UploadMediaFromReaderWithName(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.VIDEO, bytes.NewReader(media.Bytes), mediaFileName(media))
				if err != nil {
					return nil, err
				}
				msg.AddVideo(video)
			} else if looksLikeStoredMediaRef(media.Path) {
				msg.AddVideo(&schemes.UploadedInfo{Token: media.Path})
			} else {
				video, err := a.api.Uploads.UploadMediaFromFile(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.VIDEO, media.Path)
				if err != nil {
					return nil, err
				}
				msg.AddVideo(video)
			}
		case bot.OutMediaAudio:
			if len(media.Bytes) > 0 {
				audio, err := a.api.Uploads.UploadMediaFromReaderWithName(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.AUDIO, bytes.NewReader(media.Bytes), mediaFileName(media))
				if err != nil {
					return nil, err
				}
				msg.AddAudio(audio)
			} else if looksLikeStoredMediaRef(media.Path) {
				msg.AddAudio(&schemes.UploadedInfo{Token: media.Path})
			} else {
				audio, err := a.api.Uploads.UploadMediaFromFile(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.AUDIO, media.Path)
				if err != nil {
					return nil, err
				}
				msg.AddAudio(audio)
			}
		case bot.OutMediaFile:
			if len(media.Bytes) > 0 {
				file, err := a.api.Uploads.UploadMediaFromReaderWithName(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.FILE, bytes.NewReader(media.Bytes), mediaFileName(media))
				if err != nil {
					return nil, err
				}
				msg.AddFile(file)
			} else if looksLikeStoredMediaRef(media.Path) {
				msg.AddFile(&schemes.UploadedInfo{Token: media.Path})
			} else {
				file, err := a.api.Uploads.UploadMediaFromFile(apiflow.WithClass(ctx, apiflow.ClassResponse), schemes.FILE, media.Path)
				if err != nil {
					return nil, err
				}
				msg.AddFile(file)
			}
		default:
			return nil, nil
		}
		applyButtons(msg, buttons)
		return msg, nil
	}

	routes := candidateRoutes(target)
	var lastErr error
	for _, route := range routes {
		msg, err := build(route)
		if err != nil {
			log.Printf("max media build failed route=%s recipient=%s kind=%s path=%s err=%s", route.name, routeRecipient(route), media.Kind, media.Path, describeSendErr(err))
			return err
		}
		if msg == nil {
			return a.sendText(ctx, target, caption, buttons)
		}
		if err := a.sendMAX(ctx, msg); err != nil {
			lastErr = err
			log.Printf(
				"max send failed route=%s recipient=%s kind=%s caption=%q buttons=%d err=%s",
				route.name,
				routeRecipient(route),
				media.Kind,
				trimForLog(caption, 120),
				len(buttons),
				describeSendErr(err),
			)
			if !isRecipientRouteErr(err) {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (a *Adapter) sendMAX(ctx context.Context, msg *maxbot.Message) error {
	return a.api.Messages.Send(apiflow.WithClass(ctx, apiflow.ClassResponse), msg)
}

func (a *Adapter) answerCallbackFallback(ctx context.Context, text string, buttons []bot.ButtonRow, sendErr error) (bool, error) {
	if a == nil || a.api == nil {
		return false, sendErr
	}
	if !isRecipientRouteErr(sendErr) || len(buttons) > 0 || strings.TrimSpace(text) == "" {
		return false, sendErr
	}
	callbackID, ok := currentCallbackID(ctx)
	if !ok {
		return false, sendErr
	}
	_, err := a.api.Messages.AnswerOnCallback(
		apiflow.WithClass(ctx, apiflow.ClassResponse),
		callbackID,
		&schemes.CallbackAnswer{Notification: strings.TrimSpace(text)},
	)
	if err != nil {
		log.Printf("max callback answer failed callback_id=%s text=%q err=%s", callbackID, trimForLog(text, 120), describeSendErr(err))
		return true, sendErr
	}
	log.Printf("max callback answered via notification callback_id=%s text=%q", callbackID, trimForLog(text, 120))
	return true, nil
}

func applyButtons(msg *maxbot.Message, rows []bot.ButtonRow) {
	if len(rows) == 0 {
		return
	}
	kb := &maxbot.Keyboard{}
	for _, row := range rows {
		kbRow := kb.AddRow()
		for _, btn := range row {
			if btn.Kind == bot.ButtonKindURL {
				link, ok := normalizeButtonURL(btn.URL)
				if !ok {
					log.Printf("max skip invalid url button: text=%q raw_url=%q", btn.Text, btn.URL)
					continue
				}
				kbRow.AddLink(btn.Text, schemes.DEFAULT, link)
				continue
			}
			kbRow.AddCallback(btn.Text, schemes.DEFAULT, btn.ID)
		}
	}
	msg.AddKeyboard(kb)
}

func normalizeUpdate(raw schemes.UpdateInterface) (bot.Update, bool) {
	switch u := raw.(type) {
	case *schemes.MessageCreatedUpdate:
		userID := strconv.FormatInt(u.GetUserID(), 10)
		chatID := strconv.FormatInt(u.GetChatID(), 10)
		if strings.TrimSpace(chatID) == "" || chatID == "0" {
			chatID = userID
		}
		msg := normalizeIncomingMessage(u.Message.Body)
		return bot.Update{
			Platform: "max",
			ChatID:   chatID,
			UserID:   userID,
			Message:  msg,
			Raw:      raw,
			Time:     u.GetUpdateTime(),
		}, true
	case *schemes.MessageCallbackUpdate:
		userID := strconv.FormatInt(u.GetUserID(), 10)
		chatID := userID
		if u.Message != nil && u.Message.Recipient.ChatId != 0 {
			chatID = strconv.FormatInt(u.Message.Recipient.ChatId, 10)
		}
		return bot.Update{
			Platform: "max",
			ChatID:   chatID,
			UserID:   userID,
			ButtonID: strings.TrimSpace(u.Callback.Payload),
			Raw:      raw,
			Time:     u.GetUpdateTime(),
		}, true
	case *schemes.BotStartedUpdate:
		userID := strconv.FormatInt(u.GetUserID(), 10)
		chatID := strconv.FormatInt(u.GetChatID(), 10)
		if strings.TrimSpace(chatID) == "" || chatID == "0" {
			chatID = userID
		}
		text := "/start"
		payload := strings.TrimSpace(u.Payload)
		if payload == "" {
			payload = extractBotStartedPayload(u.GetDebugRaw())
		}
		if payload != "" {
			text += " " + payload
		}
		return bot.Update{
			Platform: "max",
			ChatID:   chatID,
			UserID:   userID,
			Message: bot.Message{
				Text: text,
				Kind: bot.MessageKindText,
			},
			Raw:  raw,
			Time: u.GetUpdateTime(),
		}, true
	default:
		return bot.Update{}, false
	}
}

func normalizeIncomingMessage(body schemes.MessageBody) bot.Message {
	msg := bot.Message{
		Text: body.Text,
		Kind: bot.MessageKindText,
	}
	if len(body.RawAttachments) == 0 {
		return msg
	}
	for _, raw := range body.RawAttachments {
		base := schemes.Attachment{}
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}
		switch base.Type {
		case schemes.AttachmentImage:
			var photo schemes.PhotoAttachment
			if err := json.Unmarshal(raw, &photo); err != nil {
				continue
			}
			msg.Kind = bot.MessageKindPhoto
			msg.Attachments = append(msg.Attachments, bot.Attachment{
				ID:   strings.TrimSpace(photo.Payload.Token),
				MIME: "image/*",
				URL:  strings.TrimSpace(photo.Payload.Url),
			})
		case schemes.AttachmentVideo:
			var video schemes.VideoAttachment
			if err := json.Unmarshal(raw, &video); err != nil {
				continue
			}
			msg.Kind = bot.MessageKindVideo
			msg.Attachments = append(msg.Attachments, bot.Attachment{
				ID:   strings.TrimSpace(video.Payload.Token),
				MIME: "video/*",
				URL:  strings.TrimSpace(video.Payload.Url),
			})
		case schemes.AttachmentAudio:
			var audio schemes.AudioAttachment
			if err := json.Unmarshal(raw, &audio); err != nil {
				continue
			}
			msg.Kind = bot.MessageKindAudio
			msg.Attachments = append(msg.Attachments, bot.Attachment{
				ID:   strings.TrimSpace(audio.Payload.Token),
				MIME: "audio/*",
				URL:  strings.TrimSpace(audio.Payload.Url),
			})
		case schemes.AttachmentFile:
			var file schemes.FileAttachment
			if err := json.Unmarshal(raw, &file); err != nil {
				continue
			}
			msg.Kind = bot.MessageKindFile
			msg.Attachments = append(msg.Attachments, bot.Attachment{
				ID:   strings.TrimSpace(file.Payload.Token),
				Name: strings.TrimSpace(file.Filename),
				MIME: "application/octet-stream",
				URL:  strings.TrimSpace(file.Payload.Url),
				Size: file.Size,
			})
		}
	}
	return msg
}

func extractBotStartedPayload(debugRaw string) string {
	raw := strings.TrimSpace(debugRaw)
	if raw == "" {
		return ""
	}
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ""
	}
	return findDeepLinkPayload(obj)
}

func findDeepLinkPayload(node any) string {
	switch v := node.(type) {
	case map[string]any:
		for _, key := range []string{"startPayload", "start_payload", "payload"} {
			if raw, ok := v[key]; ok {
				if s, sok := raw.(string); sok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		if msg, ok := v["message"]; ok {
			if found := findDeepLinkPayload(msg); found != "" {
				return found
			}
		}
		if at, ok := v["attaches"]; ok {
			if found := findDeepLinkPayload(at); found != "" {
				return found
			}
		}
		if at, ok := v["attachments"]; ok {
			if found := findDeepLinkPayload(at); found != "" {
				return found
			}
		}
		for _, child := range v {
			if found := findDeepLinkPayload(child); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range v {
			if found := findDeepLinkPayload(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func isRecipientRouteErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *maxbot.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == http.StatusNotFound || apiErr.Code == http.StatusBadRequest {
			return true
		}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "http 404") || strings.Contains(msg, "http 400")
}

func describeSendErr(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *maxbot.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("api code=%d message=%q details=%q", apiErr.Code, apiErr.Message, apiErr.Details)
	}
	var netErr *maxbot.NetworkError
	if errors.As(err, &netErr) {
		return fmt.Sprintf("network op=%q err=%v", netErr.Op, netErr.Err)
	}
	var timeoutErr *maxbot.TimeoutError
	if errors.As(err, &timeoutErr) {
		return fmt.Sprintf("timeout op=%q reason=%q", timeoutErr.Op, timeoutErr.Reason)
	}
	return err.Error()
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

func trimForLog(s string, max int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func applyTextFormat(msg *maxbot.Message, text string) {
	if msg == nil {
		return
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return
	}
	if strings.Contains(t, "<a ") ||
		strings.Contains(t, "</a>") ||
		strings.Contains(t, "&lt;") ||
		strings.Contains(t, "&gt;") ||
		strings.Contains(t, "<b>") ||
		strings.Contains(t, "<i>") {
		msg.SetFormat("html")
	}
}

func normalizeButtonURL(raw string) (string, bool) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", false
	}
	if strings.HasPrefix(u, "@") {
		u = "https://max.ru/" + strings.TrimPrefix(u, "@")
	}
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", false
	}
	return parsed.String(), true
}

func resolveRecipients(ctx context.Context, chatID string) (recipientIDs, error) {
	chat, err := strconv.ParseInt(strings.TrimSpace(chatID), 10, 64)
	if err != nil {
		return recipientIDs{}, err
	}
	target := recipientIDs{chat: chat, user: chat}
	if upd, ok := bot.CurrentUpdate(ctx); ok {
		if uid, uerr := strconv.ParseInt(strings.TrimSpace(upd.UserID), 10, 64); uerr == nil && uid > 0 {
			target.user = uid
		}
		target = enrichRecipientsFromCurrentUpdate(chat, upd, target)
	}
	return target, nil
}

func preferUserRoute(target recipientIDs) bool {
	if target.user <= 0 {
		return false
	}
	if target.preferUser {
		return true
	}
	return target.chatUser == 0 && (target.chat == 0 || target.chat == target.user)
}

func enrichRecipientsFromCurrentUpdate(requestedChat int64, upd bot.Update, base recipientIDs) recipientIDs {
	if upd.Platform != "max" {
		return base
	}
	currentChat, ok := parseUpdateID(upd.ChatID)
	if !ok || currentChat != requestedChat {
		return base
	}
	switch raw := upd.Raw.(type) {
	case *schemes.MessageCreatedUpdate:
		base.chat = firstNonZeroInt64(raw.Message.Recipient.ChatId, base.chat)
		base.user = firstNonZeroInt64(raw.Message.Sender.UserId, base.user)
		base.chatUser = raw.Message.Recipient.UserId
		base.preferUser = raw.Message.Recipient.ChatType == schemes.DIALOG
	case *schemes.MessageCallbackUpdate:
		base.user = firstNonZeroInt64(raw.Callback.User.UserId, base.user)
		if raw.Message != nil {
			base.chat = firstNonZeroInt64(raw.Message.Recipient.ChatId, base.chat)
			base.chatUser = raw.Message.Recipient.UserId
			base.preferUser = raw.Message.Recipient.ChatType == schemes.DIALOG
		}
	case *schemes.BotStartedUpdate:
		base.chat = firstNonZeroInt64(raw.ChatId, base.chat)
		base.user = firstNonZeroInt64(raw.User.UserId, base.user)
		base.preferUser = true
	}
	return base
}

func candidateRoutes(target recipientIDs) []recipientRoute {
	routes := make([]recipientRoute, 0, 4)
	add := func(name string, chat, user int64) {
		if chat == 0 && user == 0 {
			return
		}
		for _, item := range routes {
			if item.chat == chat && item.user == user {
				return
			}
		}
		routes = append(routes, recipientRoute{name: name, chat: chat, user: user})
	}
	if preferUserRoute(target) {
		add("user", 0, target.user)
	}
	if target.chat != 0 && target.chatUser != 0 {
		add("chat+user", target.chat, target.chatUser)
	}
	if target.chat != 0 && target.user != 0 {
		add("chat+sender", target.chat, target.user)
	}
	if target.chat != 0 {
		add("chat", target.chat, 0)
	}
	if target.user != 0 {
		add("user", 0, target.user)
	}
	return routes
}

func applyRoute(msg *maxbot.Message, route recipientRoute) {
	if msg == nil {
		return
	}
	if route.chat != 0 {
		msg.SetChat(route.chat)
	}
	if route.user != 0 {
		msg.SetUser(route.user)
	}
}

func routeRecipient(route recipientRoute) string {
	switch {
	case route.chat != 0 && route.user != 0:
		return fmt.Sprintf("chat=%d user=%d", route.chat, route.user)
	case route.chat != 0:
		return strconv.FormatInt(route.chat, 10)
	case route.user != 0:
		return strconv.FormatInt(route.user, 10)
	default:
		return "0"
	}
}

func parseUpdateID(raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func currentCallbackID(ctx context.Context) (string, bool) {
	upd, ok := bot.CurrentUpdate(ctx)
	if !ok {
		return "", false
	}
	return callbackIDFromUpdate(upd)
}

func callbackIDFromUpdate(upd bot.Update) (string, bool) {
	raw, ok := upd.Raw.(*schemes.MessageCallbackUpdate)
	if !ok {
		return "", false
	}
	callbackID := strings.TrimSpace(raw.Callback.CallbackID)
	return callbackID, callbackID != ""
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func looksLikeStoredMediaRef(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return false
	}
	if strings.Contains(path, "://") {
		return true
	}
	if strings.ContainsAny(path, `/=`) {
		return true
	}
	return !strings.HasPrefix(path, ".") && !strings.HasPrefix(path, "/")
}
