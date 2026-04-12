package db

import (
	"context"
	"time"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type MandatoryRepository struct {
	q *sqlc.Queries
}

func NewMandatoryRepository(q *sqlc.Queries) *MandatoryRepository {
	return &MandatoryRepository{q: q}
}

func (r *MandatoryRepository) Create(ctx context.Context, channelID, title, url string, requiresCheck bool) (MandatoryChannel, error) {
	row, err := r.q.CreateMandatoryChannel(ctx, sqlc.CreateMandatoryChannelParams{
		ChannelID:     channelID,
		Title:         title,
		Url:           url,
		RequiresCheck: requiresCheck,
		CreatedAt:     nowUTC(),
	})
	if err != nil {
		return MandatoryChannel{}, err
	}
	return mapMandatoryChannel(row.ID, row.ChannelID, row.Title, row.Url, row.RequiresCheck, row.Active, row.CreatedAt), nil
}

func (r *MandatoryRepository) List(ctx context.Context) ([]MandatoryChannel, error) {
	rows, err := r.q.ListMandatoryChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MandatoryChannel, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMandatoryChannel(row.ID, row.ChannelID, row.Title, row.Url, row.RequiresCheck, row.Active, row.CreatedAt))
	}
	return out, nil
}

func (r *MandatoryRepository) Get(ctx context.Context, id int64) (MandatoryChannel, error) {
	row, err := r.q.GetMandatoryChannel(ctx, id)
	if err != nil {
		return MandatoryChannel{}, err
	}
	return mapMandatoryChannel(row.ID, row.ChannelID, row.Title, row.Url, row.RequiresCheck, row.Active, row.CreatedAt), nil
}

func (r *MandatoryRepository) Delete(ctx context.Context, id int64) error {
	return r.q.DeleteMandatoryChannel(ctx, id)
}

func (r *MandatoryRepository) SetSubscription(ctx context.Context, userID string, channelRowID int64, subscribed bool) error {
	return r.q.UpsertMandatorySubscription(ctx, sqlc.UpsertMandatorySubscriptionParams{
		UserID:       userID,
		ChannelRowID: channelRowID,
		Subscribed:   subscribed,
		UpdatedAt:    nowUTC(),
	})
}

func (r *MandatoryRepository) ListUsersPendingReward(ctx context.Context, channelRowID int64) ([]string, error) {
	return r.q.ListUsersPendingMandatoryReward(ctx, sqlc.ListUsersPendingMandatoryRewardParams{
		ChannelRowID:   channelRowID,
		ChannelRowID_2: channelRowID,
	})
}

func (r *MandatoryRepository) ClaimRewardProgress(ctx context.Context, channelRowID int64, userID string) (bool, error) {
	rows, err := r.q.ClaimMandatoryRewardProgress(ctx, sqlc.ClaimMandatoryRewardProgressParams{
		ChannelRowID: channelRowID,
		UserID:       userID,
		ProcessedAt:  nowUTC(),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *MandatoryRepository) ReleaseRewardProgress(ctx context.Context, channelRowID int64, userID string) error {
	return r.q.ReleaseMandatoryRewardProgress(ctx, sqlc.ReleaseMandatoryRewardProgressParams{
		ChannelRowID: channelRowID,
		UserID:       userID,
	})
}

func (r *MandatoryRepository) ResetRewardProgress(ctx context.Context, channelRowID int64) error {
	return r.q.ResetMandatoryRewardProgress(ctx, channelRowID)
}

func mapMandatoryChannel(id int64, channelID, title, url string, requiresCheck, active bool, createdAt string) MandatoryChannel {
	return MandatoryChannel{
		ID:            id,
		ChannelID:     channelID,
		Title:         title,
		URL:           url,
		RequiresCheck: requiresCheck,
		Active:        active,
		CreatedAt:     parseTime(createdAt),
	}
}

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, raw)
	if t.IsZero() {
		t, _ = time.Parse(time.RFC3339, raw)
	}
	return t
}
