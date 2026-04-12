package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type TaskRepository struct {
	store *Store
	q     *sqlc.Queries
}

func NewTaskRepository(store *Store, q *sqlc.Queries) *TaskRepository {
	return &TaskRepository{store: store, q: q}
}

func (r *TaskRepository) Create(ctx context.Context, reward int64, channels []TaskChannel) (Task, error) {
	channels, err := normalizeTaskChannels(channels)
	if err != nil {
		return Task{}, err
	}
	reward = normalizeTaskReward(reward)

	var taskID int64
	err = r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		now := nowUTC()
		row, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
			Reward:    reward,
			Active:    true,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		taskID = row.ID
		return insertTaskChannels(ctx, q, taskID, channels)
	})
	if err != nil {
		return Task{}, err
	}
	return r.Get(ctx, taskID)
}

func (r *TaskRepository) Update(ctx context.Context, taskID int64, reward int64, channels []TaskChannel) (Task, error) {
	channels, err := normalizeTaskChannels(channels)
	if err != nil {
		return Task{}, err
	}
	reward = normalizeTaskReward(reward)

	err = r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		affected, err := q.UpdateTask(ctx, sqlc.UpdateTaskParams{
			Reward:    reward,
			Active:    true,
			UpdatedAt: nowUTC(),
			ID:        taskID,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return sql.ErrNoRows
		}
		if err := q.DeleteTaskChannelsByTaskID(ctx, taskID); err != nil {
			return err
		}
		return insertTaskChannels(ctx, q, taskID, channels)
	})
	if err != nil {
		return Task{}, err
	}
	return r.Get(ctx, taskID)
}

func (r *TaskRepository) Delete(ctx context.Context, taskID int64) (bool, error) {
	affected, err := r.q.SetTaskActive(ctx, sqlc.SetTaskActiveParams{
		Active:    false,
		UpdatedAt: nowUTC(),
		ID:        taskID,
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *TaskRepository) List(ctx context.Context) ([]Task, error) {
	todayStart, todayEnd, yesterdayStart, yesterdayEnd := dayBoundsUTC()
	rows, err := r.q.ListTasksWithStats(ctx, sqlc.ListTasksWithStatsParams{
		RewardedAt:   todayStart,
		RewardedAt_2: todayEnd,
		RewardedAt_3: yesterdayStart,
		RewardedAt_4: yesterdayEnd,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		task, err := r.loadTaskWithChannels(ctx, mapTaskWithStats(row))
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

func (r *TaskRepository) Get(ctx context.Context, taskID int64) (Task, error) {
	todayStart, todayEnd, yesterdayStart, yesterdayEnd := dayBoundsUTC()
	row, err := r.q.GetTaskWithStats(ctx, sqlc.GetTaskWithStatsParams{
		RewardedAt:   todayStart,
		RewardedAt_2: todayEnd,
		RewardedAt_3: yesterdayStart,
		RewardedAt_4: yesterdayEnd,
		ID:           taskID,
	})
	if err != nil {
		return Task{}, err
	}
	return r.loadTaskWithChannels(ctx, mapGetTaskWithStats(row))
}

func (r *TaskRepository) ListPending(ctx context.Context, userID string) ([]Task, error) {
	tasks, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		hasReward, err := r.HasReward(ctx, userID, task.ID)
		if err != nil {
			return nil, err
		}
		if hasReward {
			continue
		}
		out = append(out, task)
	}
	return out, nil
}

func (r *TaskRepository) NextPending(ctx context.Context, userID string) (Task, bool, error) {
	tasks, err := r.List(ctx)
	if err != nil {
		return Task{}, false, err
	}
	for _, task := range tasks {
		hasReward, err := r.HasReward(ctx, userID, task.ID)
		if err != nil {
			return Task{}, false, err
		}
		if hasReward {
			continue
		}
		return task, true, nil
	}
	return Task{}, false, nil
}

func (r *TaskRepository) HasReward(ctx context.Context, userID string, taskID int64) (bool, error) {
	count, err := r.q.CountTaskRewardForUser(ctx, sqlc.CountTaskRewardForUserParams{
		TaskID: taskID,
		UserID: strings.TrimSpace(userID),
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *TaskRepository) GrantReward(ctx context.Context, userID string, taskID int64) (int64, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, false, errors.New("user id is empty")
	}

	var (
		reward  int64
		granted bool
	)
	err := r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		task, err := q.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		if !task.Active {
			return sql.ErrNoRows
		}
		reward = task.Reward

		affected, err := q.InsertTaskReward(ctx, sqlc.InsertTaskRewardParams{
			TaskID:     taskID,
			UserID:     userID,
			Reward:     reward,
			RewardedAt: nowUTC(),
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		granted = true
		if err := q.AddUserCoins(ctx, sqlc.AddUserCoinsParams{
			Coins:     reward,
			UpdatedAt: nowUTC(),
			UserID:    userID,
		}); err != nil {
			return err
		}
		return q.CreateMetricEvent(ctx, sqlc.CreateMetricEventParams{
			Kind:      MetricTaskRewardGranted,
			UserID:    userID,
			RefID:     taskID,
			Value:     reward,
			CreatedAt: nowUTC(),
		})
	})
	if err != nil {
		return 0, false, err
	}
	return reward, granted, nil
}

func (r *TaskRepository) loadTaskWithChannels(ctx context.Context, task Task) (Task, error) {
	rows, err := r.q.ListTaskChannelsByTaskID(ctx, task.ID)
	if err != nil {
		return Task{}, err
	}
	task.Channels = make([]TaskChannel, 0, len(rows))
	for _, row := range rows {
		task.Channels = append(task.Channels, mapTaskChannel(row))
	}
	return task, nil
}

func insertTaskChannels(ctx context.Context, q *sqlc.Queries, taskID int64, channels []TaskChannel) error {
	now := nowUTC()
	for i, channel := range channels {
		if err := q.InsertTaskChannel(ctx, sqlc.InsertTaskChannelParams{
			TaskID:        taskID,
			ChannelID:     channel.ChannelID,
			Title:         channel.Title,
			Url:           channel.URL,
			RequiresCheck: channel.RequiresCheck,
			SortOrder:     int64(i),
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func mapTask(row sqlc.Task) Task {
	return Task{
		ID:        row.ID,
		Reward:    row.Reward,
		Active:    row.Active,
		CreatedAt: parseTime(row.CreatedAt),
		UpdatedAt: parseTime(row.UpdatedAt),
	}
}

func mapTaskWithStats(row sqlc.ListTasksWithStatsRow) Task {
	return Task{
		ID:                 row.ID,
		Reward:             row.Reward,
		Active:             row.Active,
		CreatedAt:          parseTime(row.CreatedAt),
		UpdatedAt:          parseTime(row.UpdatedAt),
		CompletedTotal:     row.CompletedTotal,
		CompletedToday:     row.CompletedToday,
		CompletedYesterday: row.CompletedYesterday,
	}
}

func mapGetTaskWithStats(row sqlc.GetTaskWithStatsRow) Task {
	return Task{
		ID:                 row.ID,
		Reward:             row.Reward,
		Active:             row.Active,
		CreatedAt:          parseTime(row.CreatedAt),
		UpdatedAt:          parseTime(row.UpdatedAt),
		CompletedTotal:     row.CompletedTotal,
		CompletedToday:     row.CompletedToday,
		CompletedYesterday: row.CompletedYesterday,
	}
}

func mapTaskChannel(row sqlc.TaskChannel) TaskChannel {
	return TaskChannel{
		ID:            row.ID,
		TaskID:        row.TaskID,
		ChannelID:     row.ChannelID,
		Title:         row.Title,
		URL:           row.Url,
		RequiresCheck: row.RequiresCheck,
		SortOrder:     row.SortOrder,
		CreatedAt:     parseTime(row.CreatedAt),
		UpdatedAt:     parseTime(row.UpdatedAt),
	}
}

func normalizeTaskReward(reward int64) int64 {
	if reward <= 0 {
		return 1
	}
	return reward
}

func normalizeTaskChannels(channels []TaskChannel) ([]TaskChannel, error) {
	out := make([]TaskChannel, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for i, channel := range channels {
		item := TaskChannel{
			ChannelID:     strings.TrimSpace(channel.ChannelID),
			Title:         strings.TrimSpace(channel.Title),
			URL:           strings.TrimSpace(channel.URL),
			RequiresCheck: channel.RequiresCheck,
			SortOrder:     int64(i),
		}
		if item.ChannelID == "" || item.Title == "" || item.URL == "" {
			return nil, errors.New("task channel fields are empty")
		}
		if _, err := url.ParseRequestURI(item.URL); err != nil {
			return nil, fmt.Errorf("invalid task channel url %q: %w", item.URL, err)
		}
		key := item.ChannelID + "|" + item.URL
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate task channel %q", item.ChannelID)
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, errors.New("task channels are empty")
	}
	return out, nil
}
