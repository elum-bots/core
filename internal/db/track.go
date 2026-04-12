package db

import (
	"context"
	"database/sql"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type TrackRepository struct {
	store *Store
	q     *sqlc.Queries
}

func NewTrackRepository(store *Store, q *sqlc.Queries) *TrackRepository {
	return &TrackRepository{store: store, q: q}
}

func (r *TrackRepository) Create(ctx context.Context, code, label, createdByUserID string) (TrackLink, error) {
	var out TrackLink
	err := r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		row, err := q.CreateTrackLink(ctx, sqlc.CreateTrackLinkParams{
			Code:            code,
			Label:           label,
			CreatedByUserID: createdByUserID,
			CreatedAt:       nowUTC(),
		})
		if err != nil {
			return err
		}
		if err := q.CreateTrackLinkStats(ctx, sqlc.CreateTrackLinkStatsParams{
			LinkID:    row.ID,
			UpdatedAt: nowUTC(),
		}); err != nil {
			return err
		}
		out = mapTrackLink(row.ID, row.Code, row.Label, row.CreatedByUserID, row.CreatedAt, 0, 0)
		return nil
	})
	return out, err
}

func (r *TrackRepository) List(ctx context.Context) ([]TrackLink, error) {
	rows, err := r.q.ListTrackLinks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TrackLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapTrackLink(row.ID, row.Code, row.Label, row.CreatedByUserID, row.CreatedAt, row.ArrivalsCount, row.GeneratedUsersCount))
	}
	return out, nil
}

func (r *TrackRepository) Get(ctx context.Context, id int64) (TrackLink, error) {
	row, err := r.q.GetTrackLink(ctx, id)
	if err != nil {
		return TrackLink{}, err
	}
	return mapTrackLink(row.ID, row.Code, row.Label, row.CreatedByUserID, row.CreatedAt, row.ArrivalsCount, row.GeneratedUsersCount), nil
}

func (r *TrackRepository) Delete(ctx context.Context, id int64) error {
	return r.q.DeleteTrackLink(ctx, id)
}

func (r *TrackRepository) MarkVisitByCode(ctx context.Context, userID, code string) (bool, error) {
	row, err := r.q.GetTrackLinkByCode(ctx, code)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	var created bool
	err = r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		affected, err := q.InsertTrackVisit(ctx, sqlc.InsertTrackVisitParams{
			UserID:    userID,
			LinkID:    row.ID,
			VisitedAt: nowUTC(),
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		created = true
		return q.IncrementTrackArrivals(ctx, sqlc.IncrementTrackArrivalsParams{
			UpdatedAt: nowUTC(),
			LinkID:    row.ID,
		})
	})
	return created, err
}

func (r *TrackRepository) MarkGeneratedByUser(ctx context.Context, userID string, linkID int64) (bool, error) {
	var created bool
	err := r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		affected, err := q.InsertTrackGeneratedUser(ctx, sqlc.InsertTrackGeneratedUserParams{
			UserID:      userID,
			LinkID:      linkID,
			GeneratedAt: nowUTC(),
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		created = true
		return q.IncrementTrackGeneratedUsers(ctx, sqlc.IncrementTrackGeneratedUsersParams{
			UpdatedAt: nowUTC(),
			LinkID:    linkID,
		})
	})
	return created, err
}

func mapTrackLink(id int64, code, label, createdByUserID, createdAt string, arrivalsCount, generatedUsersCount int64) TrackLink {
	return TrackLink{
		ID:                  id,
		Code:                code,
		Label:               label,
		CreatedByUserID:     createdByUserID,
		CreatedAt:           parseTime(createdAt),
		ArrivalsCount:       arrivalsCount,
		GeneratedUsersCount: generatedUsersCount,
	}
}
