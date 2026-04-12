package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

const (
	broadcastTypePost            = "post"
	broadcastTypeMandatoryReward = "mandatory_reward"

	broadcastStatusRunning         = "running"
	broadcastStatusCancelRequested = "cancel_requested"
	broadcastStatusCompleted       = "completed"
	broadcastStatusCanceled        = "canceled"
	broadcastStatusFailed          = "failed"
)

type BroadcastRunner struct {
	baseCtx context.Context
	bot     *elumbot.Bot
	store   *db.Store

	mu     sync.Mutex
	active map[int64]context.CancelFunc
}

type postBroadcastPayload struct {
	PostID int64 `json:"post_id"`
}

type mandatoryRewardPayload struct {
	ChannelRowID int64 `json:"channel_row_id"`
	Reset        bool  `json:"reset"`
}

type broadcastTargetResult struct {
	errText string
}

func NewBroadcastRunner(ctx context.Context, b *elumbot.Bot, store *db.Store) *BroadcastRunner {
	if ctx == nil {
		ctx = context.Background()
	}
	return &BroadcastRunner{
		baseCtx: ctx,
		bot:     b,
		store:   store,
		active:  make(map[int64]context.CancelFunc),
	}
}

func (r *BroadcastRunner) Resume(ctx context.Context) error {
	if r == nil || r.store == nil {
		return nil
	}
	items, err := r.store.Broadcasts.ListResumable(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		r.launch(item.ID)
	}
	return nil
}

func (r *BroadcastRunner) StartPost(ctx context.Context, adminChatID string, postID int64, userIDs []string) (db.BroadcastStat, error) {
	if r == nil || r.store == nil {
		return db.BroadcastStat{}, errors.New("broadcast runner is not initialized")
	}
	payload, err := json.Marshal(postBroadcastPayload{PostID: postID})
	if err != nil {
		return db.BroadcastStat{}, err
	}
	stat, err := r.store.Broadcasts.Create(ctx, broadcastTypePost, adminChatID, string(payload), userIDs)
	if err != nil {
		return db.BroadcastStat{}, err
	}
	r.launch(stat.ID)
	return stat, nil
}

func (r *BroadcastRunner) StartMandatoryReward(ctx context.Context, adminChatID string, channel db.MandatoryChannel, reset bool, userIDs []string) (db.BroadcastStat, error) {
	if r == nil || r.store == nil {
		return db.BroadcastStat{}, errors.New("broadcast runner is not initialized")
	}
	payload, err := json.Marshal(mandatoryRewardPayload{
		ChannelRowID: channel.ID,
		Reset:        reset,
	})
	if err != nil {
		return db.BroadcastStat{}, err
	}
	stat, err := r.store.Broadcasts.Create(ctx, broadcastTypeMandatoryReward, adminChatID, string(payload), userIDs)
	if err != nil {
		return db.BroadcastStat{}, err
	}
	r.launch(stat.ID)
	return stat, nil
}

func (r *BroadcastRunner) RequestStop(ctx context.Context, id int64) (db.BroadcastStat, bool, error) {
	if r == nil || r.store == nil {
		return db.BroadcastStat{}, false, errors.New("broadcast runner is not initialized")
	}
	item, err := r.store.Broadcasts.Get(ctx, id)
	if err != nil {
		return db.BroadcastStat{}, false, err
	}
	if !item.Active {
		return item, false, nil
	}
	requested, err := r.store.Broadcasts.RequestStop(ctx, id)
	if err != nil {
		return db.BroadcastStat{}, false, err
	}
	if requested {
		r.cancel(id)
	}
	item, err = r.store.Broadcasts.Get(ctx, id)
	if err != nil {
		return db.BroadcastStat{}, false, err
	}
	return item, requested, nil
}

func (r *BroadcastRunner) launch(id int64) {
	if r == nil || r.bot == nil || r.store == nil || id <= 0 {
		return
	}

	r.mu.Lock()
	if _, exists := r.active[id]; exists {
		r.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(r.baseCtx)
	r.active[id] = cancel
	r.mu.Unlock()

	task := func() {
		defer r.release(id)
		r.run(runCtx, id)
	}
	if err := r.bot.Submit(task); err != nil {
		go task()
	}
}

func (r *BroadcastRunner) cancel(id int64) {
	r.mu.Lock()
	cancel := r.active[id]
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *BroadcastRunner) release(id int64) {
	r.mu.Lock()
	delete(r.active, id)
	r.mu.Unlock()
}

func (r *BroadcastRunner) run(ctx context.Context, id int64) {
	job, err := r.store.Broadcasts.Get(ctx, id)
	if err != nil || !job.Active {
		return
	}

	switch job.Type {
	case broadcastTypePost:
		err = r.runPost(ctx, job)
	case broadcastTypeMandatoryReward:
		err = r.runMandatoryReward(ctx, job)
	default:
		err = fmt.Errorf("unsupported broadcast type: %s", job.Type)
	}
	if err != nil {
		r.fail(job.ID, job.AdminChatID, err)
	}
}

func (r *BroadcastRunner) runPost(ctx context.Context, job db.BroadcastStat) error {
	var payload postBroadcastPayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("decode post payload: %w", err)
	}
	post, err := r.store.Posts.Get(ctx, payload.PostID)
	if err != nil {
		return err
	}
	opts := makePostSendOptions(post)
	return r.processTargets(ctx, job.ID, func(ctx context.Context, userID string) broadcastTargetResult {
		if err := r.bot.Send(ctx, userID, post.Text, opts...); err != nil {
			_ = r.store.Posts.SaveDelivery(ctx, post.ID, userID, "failed", err.Error())
			_ = r.store.Metrics.Record(ctx, db.MetricPostFailed, userID, post.ID, 1)
			return broadcastTargetResult{errText: err.Error()}
		}
		_ = r.store.Posts.SaveDelivery(ctx, post.ID, userID, "sent", "")
		_ = r.store.Metrics.Record(ctx, db.MetricPostSent, userID, post.ID, 1)
		return broadcastTargetResult{}
	})
}

func (r *BroadcastRunner) runMandatoryReward(ctx context.Context, job db.BroadcastStat) error {
	var payload mandatoryRewardPayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("decode mandatory_reward payload: %w", err)
	}
	channel, err := r.store.Mandatory.Get(ctx, payload.ChannelRowID)
	if err != nil {
		return err
	}
	reward := mandatoryRewardCoins()
	return r.processTargets(ctx, job.ID, func(ctx context.Context, userID string) broadcastTargetResult {
		claimed, err := r.store.Mandatory.ClaimRewardProgress(ctx, channel.ID, userID)
		if err != nil {
			return broadcastTargetResult{errText: err.Error()}
		}
		if !claimed {
			return broadcastTargetResult{}
		}
		if err := r.store.Users.AddCoins(ctx, userID, reward); err != nil {
			_ = r.store.Mandatory.ReleaseRewardProgress(ctx, channel.ID, userID)
			return broadcastTargetResult{errText: err.Error()}
		}
		text := fmt.Sprintf("Похоже, вы не подтвердили подписку на обязательный канал.\n\nНачислил +%d монет и отправил напоминание. Подпишитесь снова, чтобы не пропустить новые сообщения.", reward)
		if err := r.bot.Send(ctx, userID, text); err != nil {
			_ = r.store.Mandatory.ReleaseRewardProgress(ctx, channel.ID, userID)
			return broadcastTargetResult{errText: err.Error()}
		}
		_ = r.store.Metrics.Record(ctx, db.MetricMandatoryRewardGranted, userID, channel.ID, reward)
		return broadcastTargetResult{}
	})
}

func (r *BroadcastRunner) processTargets(ctx context.Context, broadcastID int64, fn func(context.Context, string) broadcastTargetResult) error {
	for {
		if r.baseCtx.Err() != nil {
			return nil
		}

		job, err := r.store.Broadcasts.Get(ctx, broadcastID)
		if err != nil {
			return err
		}
		if !job.Active {
			return nil
		}
		if job.StopRequested {
			return r.finishCanceled(broadcastID, job.AdminChatID)
		}

		targets, err := r.store.Broadcasts.ListPendingTargets(ctx, broadcastID, 100)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return r.finishCompleted(broadcastID, job.AdminChatID)
		}

		for _, target := range targets {
			if r.baseCtx.Err() != nil {
				return nil
			}
			if ctx.Err() != nil {
				return r.finishCanceled(broadcastID, job.AdminChatID)
			}

			job, err = r.store.Broadcasts.Get(ctx, broadcastID)
			if err != nil {
				return err
			}
			if !job.Active {
				return nil
			}
			if job.StopRequested {
				return r.finishCanceled(broadcastID, job.AdminChatID)
			}

			result := fn(ctx, target.UserID)
			if r.baseCtx.Err() != nil {
				return nil
			}
			if ctx.Err() != nil {
				return r.finishCanceled(broadcastID, job.AdminChatID)
			}

			if result.errText != "" {
				marked, err := r.store.Broadcasts.MarkTargetError(ctx, broadcastID, target.UserID, result.errText)
				if err != nil {
					return err
				}
				if marked {
					_ = r.store.Broadcasts.IncError(ctx, broadcastID)
				}
				continue
			}

			marked, err := r.store.Broadcasts.MarkTargetSent(ctx, broadcastID, target.UserID)
			if err != nil {
				return err
			}
			if marked {
				_ = r.store.Broadcasts.IncSuccess(ctx, broadcastID)
			}
		}
	}
}

func (r *BroadcastRunner) finishCompleted(id int64, adminChatID string) error {
	ctx := context.Background()
	if err := r.store.Broadcasts.Finish(ctx, id, broadcastStatusCompleted); err != nil {
		return err
	}
	job, err := r.store.Broadcasts.Get(ctx, id)
	if err == nil {
		r.notify(job, "")
	}
	return nil
}

func (r *BroadcastRunner) finishCanceled(id int64, adminChatID string) error {
	ctx := context.Background()
	if err := r.store.Broadcasts.Finish(ctx, id, broadcastStatusCanceled); err != nil {
		return err
	}
	job, err := r.store.Broadcasts.Get(ctx, id)
	if err == nil {
		r.notify(job, "")
	}
	return nil
}

func (r *BroadcastRunner) fail(id int64, adminChatID string, runErr error) {
	ctx := context.Background()
	_ = r.store.Broadcasts.Finish(ctx, id, broadcastStatusFailed)
	job, err := r.store.Broadcasts.Get(ctx, id)
	if err != nil {
		job = db.BroadcastStat{ID: id, AdminChatID: adminChatID, Type: "unknown", Status: broadcastStatusFailed}
	}
	r.notify(job, runErr.Error())
}

func (r *BroadcastRunner) notify(job db.BroadcastStat, errText string) {
	if r == nil || r.bot == nil || strings.TrimSpace(job.AdminChatID) == "" {
		return
	}
	text := r.summaryText(job, errText)
	if strings.TrimSpace(text) == "" {
		return
	}
	_ = r.bot.Send(context.Background(), job.AdminChatID, text)
}

func (r *BroadcastRunner) summaryText(job db.BroadcastStat, errText string) string {
	switch job.Type {
	case broadcastTypePost:
		return r.postSummaryText(job, errText)
	case broadcastTypeMandatoryReward:
		return r.mandatorySummaryText(job, errText)
	default:
		if errText != "" {
			return fmt.Sprintf("Рассылка #%d завершилась с ошибкой: %s", job.ID, errText)
		}
		return fmt.Sprintf("Рассылка #%d завершена. Статус: %s", job.ID, job.Status)
	}
}

func (r *BroadcastRunner) postSummaryText(job db.BroadcastStat, errText string) string {
	var payload postBroadcastPayload
	_ = json.Unmarshal([]byte(job.PayloadJSON), &payload)

	title := "Рассылка завершена."
	switch job.Status {
	case broadcastStatusCanceled:
		title = "Рассылка остановлена."
	case broadcastStatusFailed:
		title = "Рассылка завершилась с ошибкой."
	}

	lines := []string{
		title,
		fmt.Sprintf("ID: %d", job.ID),
		fmt.Sprintf("Пост: #%d", payload.PostID),
		fmt.Sprintf("Всего: %d", job.Total),
		fmt.Sprintf("Успешно: %d", job.Success),
		fmt.Sprintf("Ошибок: %d", job.Error),
	}
	if errText != "" {
		lines = append(lines, "Ошибка: "+errText)
	}
	return strings.Join(lines, "\n")
}

func (r *BroadcastRunner) mandatorySummaryText(job db.BroadcastStat, errText string) string {
	var payload mandatoryRewardPayload
	_ = json.Unmarshal([]byte(job.PayloadJSON), &payload)

	channelTitle := fmt.Sprintf("#%d", payload.ChannelRowID)
	channelID := ""
	if payload.ChannelRowID > 0 {
		channel, err := r.store.Mandatory.Get(context.Background(), payload.ChannelRowID)
		if err == nil {
			channelTitle = channel.Title
			channelID = channel.ChannelID
		}
	}

	title := "Кампания по каналу завершена."
	switch job.Status {
	case broadcastStatusCanceled:
		title = "Кампания по каналу остановлена."
	case broadcastStatusFailed:
		title = "Кампания по каналу завершилась с ошибкой."
	}

	lines := []string{
		title,
		fmt.Sprintf("ID: %d", job.ID),
		fmt.Sprintf("Канал: %s (%s)", channelTitle, channelID),
		fmt.Sprintf("Сброс прогресса: %t", payload.Reset),
		"",
		fmt.Sprintf("Всего: %d", job.Total),
		fmt.Sprintf("Успешно: %d", job.Success),
		fmt.Sprintf("Ошибок: %d", job.Error),
	}
	if errText != "" {
		lines = append(lines, "Ошибка: "+errText)
	}
	return strings.Join(lines, "\n")
}
