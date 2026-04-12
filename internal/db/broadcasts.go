package db

import (
	"context"
	"database/sql"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type BroadcastRepository struct {
	store *Store
	q     *sqlc.Queries
}

func NewBroadcastRepository(store *Store, q *sqlc.Queries) *BroadcastRepository {
	return &BroadcastRepository{store: store, q: q}
}

func (r *BroadcastRepository) Start(ctx context.Context, typ string, total int64) (BroadcastStat, error) {
	return r.startWithQueries(ctx, r.q, typ, total, "", "{}")
}

func (r *BroadcastRepository) Create(ctx context.Context, typ, adminChatID, payloadJSON string, userIDs []string) (BroadcastStat, error) {
	if r.store == nil {
		return BroadcastStat{}, sql.ErrConnDone
	}

	var stat BroadcastStat
	err := r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		row, err := r.startWithQueries(ctx, q, typ, int64(len(userIDs)), adminChatID, payloadJSON)
		if err != nil {
			return err
		}
		stat = row
		for i, userID := range userIDs {
			if err := q.CreateBroadcastTarget(ctx, sqlc.CreateBroadcastTargetParams{
				BroadcastID: stat.ID,
				UserID:      userID,
				SortOrder:   int64(i),
				UpdatedAt:   nowUTC(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return BroadcastStat{}, err
	}
	return stat, nil
}

func (r *BroadcastRepository) startWithQueries(ctx context.Context, q *sqlc.Queries, typ string, total int64, adminChatID, payloadJSON string) (BroadcastStat, error) {
	row, err := q.StartBroadcastStats(ctx, sqlc.StartBroadcastStatsParams{
		Date:        nowUTC(),
		Type:        typ,
		Total:       total,
		AdminChatID: adminChatID,
		PayloadJson: payloadJSON,
		CreatedAt:   nowUTC(),
		UpdatedAt:   nowUTC(),
	})
	if err != nil {
		return BroadcastStat{}, err
	}
	return mapBroadcastStat(row.ID, row.Date, row.Type, row.Total, row.Success, row.Error, row.Active, row.Status, row.StopRequested, row.AdminChatID, row.PayloadJson, row.CreatedAt, row.UpdatedAt, row.FinishedAt), nil
}

func (r *BroadcastRepository) IncSuccess(ctx context.Context, id int64) error {
	return r.q.IncBroadcastSuccess(ctx, sqlc.IncBroadcastSuccessParams{
		UpdatedAt: nowUTC(),
		ID:        id,
	})
}

func (r *BroadcastRepository) IncError(ctx context.Context, id int64) error {
	return r.q.IncBroadcastError(ctx, sqlc.IncBroadcastErrorParams{
		UpdatedAt: nowUTC(),
		ID:        id,
	})
}

func (r *BroadcastRepository) Finish(ctx context.Context, id int64, status string) error {
	now := nowUTC()
	return r.q.FinishBroadcastStats(ctx, sqlc.FinishBroadcastStatsParams{
		Status:    status,
		UpdatedAt: now,
		FinishedAt: sql.NullString{
			String: now,
			Valid:  true,
		},
		ID: id,
	})
}

func (r *BroadcastRepository) Get(ctx context.Context, id int64) (BroadcastStat, error) {
	row, err := r.q.GetBroadcastStats(ctx, id)
	if err != nil {
		return BroadcastStat{}, err
	}
	return mapBroadcastStat(row.ID, row.Date, row.Type, row.Total, row.Success, row.Error, row.Active, row.Status, row.StopRequested, row.AdminChatID, row.PayloadJson, row.CreatedAt, row.UpdatedAt, row.FinishedAt), nil
}

func (r *BroadcastRepository) RequestStop(ctx context.Context, id int64) (bool, error) {
	rows, err := r.q.RequestStopBroadcastStats(ctx, sqlc.RequestStopBroadcastStatsParams{
		UpdatedAt: nowUTC(),
		ID:        id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *BroadcastRepository) List(ctx context.Context, limit int64) ([]BroadcastStat, error) {
	rows, err := r.q.ListBroadcastStats(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]BroadcastStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapBroadcastStat(row.ID, row.Date, row.Type, row.Total, row.Success, row.Error, row.Active, row.Status, row.StopRequested, row.AdminChatID, row.PayloadJson, row.CreatedAt, row.UpdatedAt, row.FinishedAt))
	}
	return items, nil
}

func (r *BroadcastRepository) Active(ctx context.Context) ([]BroadcastStat, error) {
	rows, err := r.q.ListActiveBroadcastStats(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]BroadcastStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapBroadcastStat(row.ID, row.Date, row.Type, row.Total, row.Success, row.Error, row.Active, row.Status, row.StopRequested, row.AdminChatID, row.PayloadJson, row.CreatedAt, row.UpdatedAt, row.FinishedAt))
	}
	return items, nil
}

func (r *BroadcastRepository) ListResumable(ctx context.Context) ([]BroadcastStat, error) {
	rows, err := r.q.ListResumableBroadcastStats(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]BroadcastStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapBroadcastStat(row.ID, row.Date, row.Type, row.Total, row.Success, row.Error, row.Active, row.Status, row.StopRequested, row.AdminChatID, row.PayloadJson, row.CreatedAt, row.UpdatedAt, row.FinishedAt))
	}
	return items, nil
}

func (r *BroadcastRepository) ListPendingTargets(ctx context.Context, broadcastID, limit int64) ([]BroadcastTarget, error) {
	rows, err := r.q.ListPendingBroadcastTargets(ctx, sqlc.ListPendingBroadcastTargetsParams{
		BroadcastID: broadcastID,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]BroadcastTarget, 0, len(rows))
	for _, row := range rows {
		items = append(items, BroadcastTarget{
			BroadcastID: row.BroadcastID,
			UserID:      row.UserID,
			SortOrder:   row.SortOrder,
			Status:      row.Status,
			Error:       row.Error,
			UpdatedAt:   parseTime(row.UpdatedAt),
		})
	}
	return items, nil
}

func (r *BroadcastRepository) MarkTargetSent(ctx context.Context, broadcastID int64, userID string) (bool, error) {
	rows, err := r.q.MarkBroadcastTargetSent(ctx, sqlc.MarkBroadcastTargetSentParams{
		UpdatedAt:   nowUTC(),
		BroadcastID: broadcastID,
		UserID:      userID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *BroadcastRepository) MarkTargetError(ctx context.Context, broadcastID int64, userID, errText string) (bool, error) {
	rows, err := r.q.MarkBroadcastTargetError(ctx, sqlc.MarkBroadcastTargetErrorParams{
		Error:       errText,
		UpdatedAt:   nowUTC(),
		BroadcastID: broadcastID,
		UserID:      userID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func mapBroadcastStat(id int64, date, typ string, total, success, failed int64, active bool, status string, stopRequested bool, adminChatID, payloadJSON, createdAt, updatedAt string, finishedAt sql.NullString) BroadcastStat {
	item := BroadcastStat{
		ID:            id,
		Date:          parseTime(date),
		Type:          typ,
		Total:         total,
		Success:       success,
		Error:         failed,
		Active:        active,
		Status:        status,
		StopRequested: stopRequested,
		AdminChatID:   adminChatID,
		PayloadJSON:   payloadJSON,
		CreatedAt:     parseTime(createdAt),
		UpdatedAt:     parseTime(updatedAt),
	}
	if finishedAt.Valid {
		t := parseTime(finishedAt.String)
		item.FinishedAt = &t
	}
	return item
}
