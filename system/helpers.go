package system

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
	"github.com/elum-utils/env"
)

const (
	dialogMandatoryAdd          = "system.mandatory_add"
	dialogMandatoryRewardSelect = "system.mandatory_reward_select"
	dialogMandatoryRewardReset  = "system.mandatory_reward_reset"
	dialogPostCreate            = "system.post_create"
	dialogTaskCreate            = "system.task_create"
	dialogTaskEdit              = "system.task_edit"
	dialogDeepSeekTokenAdd      = "system.deepseek_token_add"
	dialogDeepSeekTokenEdit     = "system.deepseek_token_edit"
	dialogGeminiTokenAdd        = "system.gemini_token_add"
	dialogGeminiTokenEdit       = "system.gemini_token_edit"
)

const (
	keyMandatoryChannelID     = "mandatory_channel_id"
	keyMandatoryChannelTitle  = "mandatory_channel_title"
	keyMandatoryChannelURL    = "mandatory_channel_url"
	keyMandatoryRequiresCheck = "mandatory_requires_check"
	keyMandatoryRewardRowID   = "mandatory_reward_row_id"
	keyMandatoryRewardReset   = "mandatory_reward_reset"
	keyPostTitle              = "post_title"
	keyPostText               = "post_text"
	keyPostPhoto              = "post_photo"
	keyPostButtons            = "post_buttons"
	keyTaskReward             = "task_reward"
	keyTaskChannelsRaw        = "task_channels_raw"
	keyTaskID                 = "task_id"
	keyIntegrationTokenValue  = "integration_token_value"
	keyIntegrationTokenID     = "integration_token_id"
)

const (
	infoPrivacyURL  = "https://otkrytka-s-tvoim-licom.gitbook.io/privacy-policy-1"
	infoTermsURL    = "https://otkrytka-s-tvoim-licom.gitbook.io/privacy-policy-1/terms-of-use"
	buttonTaskCheck = "task_check"
)

func requireStore(deps Dependencies) (*db.Store, error) {
	if deps.Store == nil {
		return nil, errors.New("store is not initialized")
	}
	return deps.Store, nil
}

func currentUpdate(ctx context.Context) (elumbot.Update, error) {
	upd, ok := elumbot.CurrentUpdate(ctx)
	if !ok {
		return elumbot.Update{}, errors.New("context error")
	}
	return upd, nil
}

func isAdmin(upd elumbot.Update) bool {
	userID := strings.TrimSpace(upd.UserID)
	if userID == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(upd.Platform)) {
	case "tg":
		return adminSet("ADMIN_USER_TG_IDS")[userID]
	case "max":
		return adminSet("ADMIN_USER_MAX_IDS")[userID]
	default:
		return false
	}
}

func adminSet(key string) map[string]bool {
	values := env.GetEnvArrayString(key, ",", []string{})
	out := make(map[string]bool, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out[v] = true
		}
	}
	return out
}

func helpText(includeAdmin bool, deps Dependencies) string {
	user := strings.TrimSpace(strings.Join([]string{
		"Справка по боту:",
		"",
		"Пользовательские команды:",
		"",
		"/start - базовое приветствие и регистрация пользователя.",
		"/close - закрыть текущий диалог.",
		"/balance - показать текущий баланс.",
		"/ref - показать реферальную ссылку и прогресс.",
		"/help - список команд.",
	}, "\n"))
	usage := helpInfoText(deps)
	if !includeAdmin {
		return user + "\n\n" + usage
	}
	adminParts := []string{
		"Системные команды:",
		"",
		"/broadcast_active - активные рассылки.",
		"/broadcast_stats [limit] - история запусков рассылок.",
		"/broadcast_stop <id> - остановить активную рассылку.",
		"",
		"/get_chat_id <url_канала> - найти chat_id канала/чата в MAX по URL.",
		"",
		"Посты:",
		"/post_create - создать пост в диалоге.",
		"/post_list - список постов.",
		"/post_preview <id> - отправить превью поста в личный чат.",
		"/post_send <id> - отправить пост всем пользователям.",
		"",
		"Mandatory:",
		"/mandatory_add - добавить mandatory канал в диалоге.",
		"/mandatory_list - список mandatory каналов.",
		"/mandatory_del <id> - удалить mandatory канал.",
		"/mandatory_reward [id] - запустить mandatory_reward.",
		"",
		"Track:",
		"/track_create <метка> - создать track-ссылку.",
		"/track_list - список track-ссылок и статистика.",
		"/track_get <id> - получить одну track-ссылку.",
		"/track_del <id> - удалить track-ссылку.",
		"",
		"Баланс:",
		"/balance_add <user_id> <amount> - начислить монеты.",
		"",
		"Задания:",
		"/task_create - создать задание в диалоге.",
		"/task_edit <id> - изменить задание в диалоге.",
		"/task_list - список активных заданий.",
		"",
		"Метрики:",
		"/stats - статистика бота по дням.",
	}
	if deepSeekFeatureEnabled(deps) {
		adminParts = append(adminParts,
			"",
			"DeepSeek токены:",
			"/deepseek_token_add - добавить токен.",
			"/deepseek_token_list - список токенов.",
			"/deepseek_token_edit <id> - обновить токен.",
			"/deepseek_token_del <id> - удалить токен.",
		)
	}
	if geminiFeatureEnabled(deps) {
		adminParts = append(adminParts,
			"",
			"Gemini токены:",
			"/gemini_token_add - добавить токен.",
			"/gemini_token_list - список токенов.",
			"/gemini_token_edit <id> - обновить токен.",
			"/gemini_token_del <id> - удалить токен.",
		)
	}
	admin := strings.TrimSpace(strings.Join(adminParts, "\n"))
	return user + "\n\n" + usage + "\n\n" + admin
}

func helpInfoText(deps Dependencies) string {
	if text := strings.TrimSpace(deps.HelpInfo); text != "" {
		return text
	}
	return strings.TrimSpace(strings.Join([]string{
		"Как пользоваться:",
		"",
		"(тут описание бота самого будет)",
		"",
		"Если есть вопросы — пишите clck.ru/3RFgyG",
		"",
		fmt.Sprintf("🔒 Политика конфиденциальности: <a href=\"%s\">читать</a>", infoPrivacyURL),
		fmt.Sprintf("📜 Пользовательское соглашение: <a href=\"%s\">читать</a>", infoTermsURL),
	}, "\n"))
}

func parseSingleArgInt(args []string) (int64, bool) {
	if len(args) != 1 {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parsePositiveAmount(raw string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parsePositiveInt64(raw string) (int64, bool) {
	return parsePositiveAmount(raw)
}

func parseCheckFlag(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "да", "д", "yes", "y", "true":
		return true, true
	case "0", "нет", "н", "no", "n", "false":
		return false, true
	default:
		return false, false
	}
}

func validateNonEmpty(in elumbot.Input) error {
	if strings.TrimSpace(in.Text) == "" {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validateCheckFlag(in elumbot.Input) error {
	if _, ok := parseCheckFlag(in.Text); !ok {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validatePositiveInt64(in elumbot.Input) error {
	if _, ok := parsePositiveInt64(in.Text); !ok {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validateTaskChannels(in elumbot.Input) error {
	if _, err := parseTaskChannelsInput(in.Text); err != nil {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validateTaskRewardOptional(in elumbot.Input) error {
	if isEmptyToken(in.Text) {
		return nil
	}
	return validatePositiveInt64(in)
}

func validateTaskChannelsOptional(in elumbot.Input) error {
	if isEmptyToken(in.Text) {
		return nil
	}
	return validateTaskChannels(in)
}

func validateTokenValue(in elumbot.Input) error {
	if strings.TrimSpace(in.Text) == "" {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func isEmptyToken(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "/empty")
}

func validatePostTitle(in elumbot.Input) error {
	if strings.TrimSpace(in.Text) == "" {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validatePostText(in elumbot.Input) error {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func validatePostPhoto(in elumbot.Input) error {
	if isEmptyToken(in.Text) {
		return nil
	}
	if len(in.Attachments) == 0 {
		return elumbot.ErrInvalidInput
	}
	switch in.Kind {
	case elumbot.MessageKindPhoto, elumbot.MessageKindVideo, elumbot.MessageKindAudio, elumbot.MessageKindFile:
		return nil
	default:
		return elumbot.ErrInvalidInput
	}
}

func validatePostButtons(in elumbot.Input) error {
	_, err := parsePostButtonsRaw(in.Text)
	if err != nil {
		return elumbot.ErrInvalidInput
	}
	return nil
}

func parsePostButtonsRaw(raw string) ([]db.PostButton, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || isEmptyToken(trimmed) {
		return nil, nil
	}

	lines := strings.Split(trimmed, "\n")
	out := make([]db.PostButton, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid buttons format")
		}
		text := strings.TrimSpace(parts[0])
		link := strings.TrimSpace(parts[1])
		if text == "" || link == "" {
			return nil, errors.New("invalid buttons format")
		}
		if _, err := url.ParseRequestURI(normalizeURL(link)); err != nil {
			return nil, err
		}
		out = append(out, db.PostButton{Text: text, URL: normalizeURL(link)})
	}
	return out, nil
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "@") {
		return "https://t.me/" + strings.TrimPrefix(raw, "@")
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	return "https://" + raw
}

func parseTaskChannelsInput(raw string) ([]db.TaskChannel, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	channels := make([]db.TaskChannel, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 && len(parts) != 4 {
			return nil, fmt.Errorf("неверный формат строки %q", line)
		}

		channelID := strings.TrimSpace(parts[0])
		title := strings.TrimSpace(parts[1])
		channelURL := normalizeURL(parts[2])
		if channelID == "" || title == "" || strings.TrimSpace(channelURL) == "" {
			return nil, fmt.Errorf("пустые значения в строке %q", line)
		}
		if _, err := url.ParseRequestURI(channelURL); err != nil {
			return nil, err
		}

		requiresCheck := true
		if len(parts) == 4 {
			value, ok := parseCheckFlag(parts[3])
			if !ok {
				return nil, fmt.Errorf("некорректное значение проверки в строке %q", line)
			}
			requiresCheck = value
		}

		channels = append(channels, db.TaskChannel{
			ChannelID:     channelID,
			Title:         title,
			URL:           channelURL,
			RequiresCheck: requiresCheck,
			SortOrder:     int64(len(channels)),
		})
	}
	if len(channels) == 0 {
		return nil, errors.New("список каналов пуст")
	}
	return channels, nil
}

func taskChannelsText(channels []db.TaskChannel) string {
	lines := make([]string, 0, len(channels))
	for i, channel := range channels {
		check := "1"
		if !channel.RequiresCheck {
			check = "0"
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s) | check=%s\n%s", i+1, channel.Title, channel.ChannelID, check, channel.URL))
	}
	return strings.Join(lines, "\n\n")
}

func taskButtons(channels []db.TaskChannel) []elumbot.ButtonRow {
	rows := make([]elumbot.ButtonRow, 0, len(channels)+1)
	for _, channel := range channels {
		title := strings.TrimSpace(channel.Title)
		if title == "" {
			title = strings.TrimSpace(channel.ChannelID)
		}
		if title == "" || strings.TrimSpace(channel.URL) == "" {
			continue
		}
		rows = append(rows, elumbot.Row(elumbot.URLBtn(title, channel.URL)))
	}
	rows = append(rows, elumbot.Row(elumbot.Btn(buttonTaskCheck, "✅ Проверить задание")))
	return rows
}

func taskAdminText(task db.Task) string {
	return fmt.Sprintf(
		"#%d | награда: %d | выполнено: всего=%d, сегодня=%d, вчера=%d\n\n%s",
		task.ID,
		task.Reward,
		task.CompletedTotal,
		task.CompletedToday,
		task.CompletedYesterday,
		taskChannelsText(task.Channels),
	)
}

func taskCardText(task db.Task) string {
	return fmt.Sprintf(
		"Доступное задание #%d\nНаграда: %d монет\n\nПодпишитесь на каналы ниже и нажмите кнопку проверки.\n\n%s",
		task.ID,
		task.Reward,
		taskChannelsText(task.Channels),
	)
}

func taskMissingText(task db.Task, missing []db.TaskChannel) string {
	return fmt.Sprintf(
		"Не вижу подписку на все каналы из задания #%d.\n\nПодпишитесь на недостающие каналы и нажмите проверку ещё раз.\n\n%s",
		task.ID,
		taskChannelsText(missing),
	)
}

func parsePostMedia(in elumbot.Input) (mediaID, mediaKind string) {
	if len(in.Attachments) == 0 {
		return "", ""
	}
	att := in.Attachments[0]
	switch in.Kind {
	case elumbot.MessageKindPhoto:
		return strings.TrimSpace(att.ID), "image"
	case elumbot.MessageKindVideo:
		return strings.TrimSpace(att.ID), "video"
	case elumbot.MessageKindAudio:
		return strings.TrimSpace(att.ID), "audio"
	case elumbot.MessageKindFile:
		return strings.TrimSpace(att.ID), "file"
	default:
		return "", ""
	}
}

func makePostSendOptions(post db.Post) []elumbot.SendOption {
	opts := make([]elumbot.SendOption, 0, 2)
	if len(post.Buttons) > 0 {
		rows := make([]elumbot.ButtonRow, 0, len(post.Buttons))
		for _, btn := range post.Buttons {
			rows = append(rows, elumbot.Row(elumbot.Button{
				Text: btn.Text,
				Kind: elumbot.ButtonKindURL,
				URL:  btn.URL,
			}))
		}
		opts = append(opts, elumbot.WithButtons(rows...))
	}
	if post.MediaID != "" {
		switch post.MediaKind {
		case "image":
			opts = append(opts, elumbot.WithMedia(elumbot.Image(post.MediaID)))
		case "video":
			opts = append(opts, elumbot.WithMedia(elumbot.Video(post.MediaID)))
		case "audio":
			opts = append(opts, elumbot.WithMedia(elumbot.Audio(post.MediaID)))
		case "file":
			opts = append(opts, elumbot.WithMedia(elumbot.File(post.MediaID)))
		}
	}
	return opts
}

func postBodyWithTitle(title, body string) string {
	if strings.TrimSpace(body) == "" {
		return title
	}
	return title + "\n\n" + body
}

func randomTrackCode(n int) string {
	if n <= 0 {
		return ""
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for _, v := range buf {
		b.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return b.String()
}

func parseStartTrackingCode(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	if !strings.HasPrefix(v, "trk_") {
		return ""
	}
	code := strings.TrimPrefix(v, "trk_")
	if len(code) != 8 {
		return ""
	}
	for _, ch := range code {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return ""
		}
	}
	return code
}

func trackLink(platform, code string) string {
	payload := "trk_" + strings.ToLower(strings.TrimSpace(code))
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "tg":
		if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("TG_BOT_USERNAME", "")), "@"); username != "" {
			return "https://t.me/" + username + "?start=" + payload
		}
	case "max":
		if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("MAX_BOT_USERNAME", "")), "@"); username != "" {
			return "https://max.ru/" + username + "?start=" + payload
		}
	}
	if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("TG_BOT_USERNAME", "")), "@"); username != "" {
		return "https://t.me/" + username + "?start=" + payload
	}
	if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("MAX_BOT_USERNAME", "")), "@"); username != "" {
		return "https://max.ru/" + username + "?start=" + payload
	}
	return payload
}

func referralLink(platform, userID string) string {
	payload := "ref_" + strings.TrimSpace(userID)
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "tg":
		if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("TG_BOT_USERNAME", "")), "@"); username != "" {
			return "https://t.me/" + username + "?start=" + payload
		}
	case "max":
		if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("MAX_BOT_USERNAME", "")), "@"); username != "" {
			return "https://max.ru/" + username + "?start=" + payload
		}
	}
	if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("TG_BOT_USERNAME", "")), "@"); username != "" {
		return "https://t.me/" + username + "?start=" + payload
	}
	if username := strings.TrimPrefix(strings.TrimSpace(env.GetEnvString("MAX_BOT_USERNAME", "")), "@"); username != "" {
		return "https://max.ru/" + username + "?start=" + payload
	}
	return payload
}

func parseStartReferralUserID(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	if !strings.HasPrefix(v, "ref_") {
		return ""
	}
	id := strings.TrimSpace(strings.TrimPrefix(v, "ref_"))
	if id == "" {
		return ""
	}
	for _, ch := range id {
		if !unicode.IsDigit(ch) {
			return ""
		}
	}
	return id
}

func referralRewardPerRef() float64 {
	raw := strings.TrimSpace(env.GetEnvString("REFERRAL_REWARD_PER_REF", "1"))
	if raw == "" {
		return 1
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 1
	}
	return value
}

func formatDecimal(v float64) string {
	if math.Abs(v) < 0.00001 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 6, 64), "0"), ".")
}

func mandatoryRewardCoins() int64 {
	return int64(env.GetEnvInt("MANDATORY_REWARD_COINS", 1))
}

func replyLongText(ctx context.Context, b *elumbot.Bot, chatID, text string) error {
	text = strings.TrimSpace(text)
	if len(text) <= 3500 {
		return b.Send(ctx, chatID, text)
	}
	for len(text) > 0 {
		chunk := text
		if len(chunk) > 3500 {
			chunk = chunk[:3500]
		}
		if err := b.Send(ctx, chatID, strings.TrimSpace(chunk)); err != nil {
			return err
		}
		if len(text) <= len(chunk) {
			break
		}
		text = text[len(chunk):]
	}
	return nil
}

func findMandatoryChannel(ctx context.Context, store *db.Store, id int64) (db.MandatoryChannel, bool, error) {
	items, err := store.Mandatory.List(ctx)
	if err != nil {
		return db.MandatoryChannel{}, false, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return db.MandatoryChannel{}, false, nil
}

func startAdminOnly(ctx context.Context) error {
	return elumbot.Reply(ctx, "Команда доступна только админам")
}

func startFeatureDisabled(ctx context.Context) error {
	return elumbot.Reply(ctx, "Функция выключена в env.")
}

func sqlNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func statsText(s db.BotStats) string {
	return fmt.Sprintf(
		"Статистика бота:\n\n"+
			"Уникальные:\n- Всего: %d\n- Сегодня: %d\n- Вчера: %d\n\n"+
			"Новые:\n- Сегодня: %d\n- Вчера: %d\n\n"+
			"Рефералы:\n- Всего: %d\n- Сегодня: %d\n- Вчера: %d\n\n"+
			"Track:\n- Переходов всего: %d\n- Сегодня: %d\n- Вчера: %d\n\n"+
			"Баланс:\n- Начислений всего: %d\n- Сегодня: %d\n- Вчера: %d\n\n"+
			"Рассылки:\n- Отправлено всего: %d\n- Отправлено сегодня: %d\n- Отправлено вчера: %d\n- Ошибок сегодня: %d\n- Ошибок вчера: %d\n- Всего запусков: %d\n- Активных сейчас: %d",
		s.UniqueTotal, s.UniqueToday, s.UniqueYesterday,
		s.NewUsersToday, s.NewUsersYesterday,
		s.RefTotal, s.RefToday, s.RefYesterday,
		s.TrackVisitsTotal, s.TrackVisitsToday, s.TrackVisitsYesterday,
		s.BalanceAddedTotal, s.BalanceAddedToday, s.BalanceAddedYesterday,
		s.PostSentTotal, s.PostSentToday, s.PostSentYesterday, s.PostFailedToday, s.PostFailedYesterday,
		s.BroadcastsTotal, s.BroadcastsActive,
	)
}

func broadcastStatText(item db.BroadcastStat) string {
	remaining := item.Total - item.Success - item.Error
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf(
		"#%d | %s | %s | всего=%d | успешно=%d | ошибка=%d | осталось=%d | status=%s | active=%t",
		item.ID,
		item.Date.UTC().Format("2006-01-02 15:04:05"),
		item.Type,
		item.Total,
		item.Success,
		item.Error,
		remaining,
		item.Status,
		item.Active,
	)
}

func deepSeekFeatureEnabled(deps Dependencies) bool {
	return deps.Integrations != nil && deps.Integrations.DeepSeek != nil
}

func geminiFeatureEnabled(deps Dependencies) bool {
	return deps.Integrations != nil && deps.Integrations.Gemini != nil
}

func maskToken(raw string) string {
	token := strings.TrimSpace(raw)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
