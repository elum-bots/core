package db

import (
	"context"
	"time"

	"github.com/elum-bots/core/internal/db/sqlc"
)

const (
	MetricStart                  = "start"
	MetricPostSent               = "post_sent"
	MetricPostFailed             = "post_failed"
	MetricMandatoryRewardGranted = "mandatory_reward_granted"
	MetricBalanceAdded           = "balance_added"
	MetricTaskRewardGranted      = "task_reward_granted"
)

type StatsRepository struct {
	q *sqlc.Queries
}

func NewStatsRepository(q *sqlc.Queries) *StatsRepository {
	return &StatsRepository{q: q}
}

func (r *StatsRepository) GetBotStats(ctx context.Context) (BotStats, error) {
	var out BotStats
	todayStart, todayEnd, yesterdayStart, yesterdayEnd := dayBoundsUTC()

	var err error
	if out.UniqueTotal, err = r.q.CountUsersTotal(ctx); err != nil {
		return BotStats{}, err
	}
	if out.UniqueToday, err = r.q.CountUsersUpdatedBetween(ctx, sqlc.CountUsersUpdatedBetweenParams{UpdatedAt: todayStart, UpdatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.UniqueYesterday, err = r.q.CountUsersUpdatedBetween(ctx, sqlc.CountUsersUpdatedBetweenParams{UpdatedAt: yesterdayStart, UpdatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.NewUsersToday, err = r.q.CountUsersCreatedBetween(ctx, sqlc.CountUsersCreatedBetweenParams{CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.NewUsersYesterday, err = r.q.CountUsersCreatedBetween(ctx, sqlc.CountUsersCreatedBetweenParams{CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.StartsToday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricStart, CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.StartsYesterday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricStart, CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.RefTotal, err = r.q.CountReferredUsersTotal(ctx); err != nil {
		return BotStats{}, err
	}
	if out.RefToday, err = r.q.CountReferredUsersBetween(ctx, sqlc.CountReferredUsersBetweenParams{CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.RefYesterday, err = r.q.CountReferredUsersBetween(ctx, sqlc.CountReferredUsersBetweenParams{CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.PostSentTotal, err = r.q.CountMetricEventsTotal(ctx, MetricPostSent); err != nil {
		return BotStats{}, err
	}
	if out.PostSentToday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricPostSent, CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.PostSentYesterday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricPostSent, CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.PostFailedToday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricPostFailed, CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.PostFailedYesterday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricPostFailed, CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.MandatoryRewardTotal, err = r.q.CountMetricEventsTotal(ctx, MetricMandatoryRewardGranted); err != nil {
		return BotStats{}, err
	}
	if out.MandatoryRewardToday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricMandatoryRewardGranted, CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.MandatoryRewardYesterday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricMandatoryRewardGranted, CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.TrackVisitsTotal, err = r.q.CountTrackVisitsTotal(ctx); err != nil {
		return BotStats{}, err
	}
	if out.TrackVisitsToday, err = r.q.CountTrackVisitsBetween(ctx, sqlc.CountTrackVisitsBetweenParams{VisitedAt: todayStart, VisitedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.TrackVisitsYesterday, err = r.q.CountTrackVisitsBetween(ctx, sqlc.CountTrackVisitsBetweenParams{VisitedAt: yesterdayStart, VisitedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.BalanceAddedTotal, err = r.q.CountMetricEventsTotal(ctx, MetricBalanceAdded); err != nil {
		return BotStats{}, err
	}
	if out.BalanceAddedToday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricBalanceAdded, CreatedAt: todayStart, CreatedAt_2: todayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.BalanceAddedYesterday, err = r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{Kind: MetricBalanceAdded, CreatedAt: yesterdayStart, CreatedAt_2: yesterdayEnd}); err != nil {
		return BotStats{}, err
	}
	if out.BroadcastsTotal, err = r.q.CountBroadcastStatsTotal(ctx); err != nil {
		return BotStats{}, err
	}
	if out.BroadcastsActive, err = r.q.CountBroadcastStatsActive(ctx); err != nil {
		return BotStats{}, err
	}

	return out, nil
}

func dayBoundsUTC() (todayStart, todayEnd, yesterdayStart, yesterdayEnd string) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := start.Add(-24 * time.Hour)
	next := start.Add(24 * time.Hour)
	return start.Format(time.RFC3339), next.Format(time.RFC3339), yesterday.Format(time.RFC3339), start.Format(time.RFC3339)
}
