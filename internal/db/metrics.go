package db

import (
	"context"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type MetricsRepository struct {
	q *sqlc.Queries
}

func NewMetricsRepository(q *sqlc.Queries) *MetricsRepository {
	return &MetricsRepository{q: q}
}

func (r *MetricsRepository) Record(ctx context.Context, kind, userID string, refID int64, value int64) error {
	return r.q.CreateMetricEvent(ctx, sqlc.CreateMetricEventParams{
		Kind:      kind,
		UserID:    userID,
		RefID:     refID,
		Value:     value,
		CreatedAt: nowUTC(),
	})
}

func (r *MetricsRepository) CountTotal(ctx context.Context, kind string) (int64, error) {
	return r.q.CountMetricEventsTotal(ctx, kind)
}

func (r *MetricsRepository) CountBetween(ctx context.Context, kind, start, end string) (int64, error) {
	return r.q.CountMetricEventsBetween(ctx, sqlc.CountMetricEventsBetweenParams{
		Kind:        kind,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}
