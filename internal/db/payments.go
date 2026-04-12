package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type PaymentRepository struct {
	q *sqlc.Queries
}

func NewPaymentRepository(q *sqlc.Queries) *PaymentRepository {
	return &PaymentRepository{q: q}
}

func (r *PaymentRepository) CreateTransaction(ctx context.Context, tx PaymentTransaction) (PaymentTransaction, error) {
	paidAt := sql.NullString{}
	if tx.PaidAt != nil {
		paidAt = sql.NullString{
			String: tx.PaidAt.UTC().Format(time.RFC3339Nano),
			Valid:  true,
		}
	}
	now := nowUTC()
	row, err := r.q.CreatePaymentTransaction(ctx, sqlc.CreatePaymentTransactionParams{
		TransactionID:  tx.TransactionID,
		UserID:         tx.UserID,
		PlatformUserID: tx.PlatformUserID,
		ProductKey:     tx.ProductKey,
		ProductTitle:   tx.ProductTitle,
		Coins:          tx.Coins,
		Amount:         tx.Amount,
		Currency:       tx.Currency,
		PaymentMethod:  tx.PaymentMethod,
		Status:         tx.Status,
		RedirectUrl:    tx.RedirectURL,
		Rewarded:       tx.Rewarded,
		PaidAt:         paidAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return PaymentTransaction{}, err
	}
	return mapPaymentTransaction(row), nil
}

func (r *PaymentRepository) GetByTransactionID(ctx context.Context, transactionID string) (PaymentTransaction, error) {
	row, err := r.q.GetPaymentTransactionByID(ctx, transactionID)
	if err != nil {
		return PaymentTransaction{}, err
	}
	return mapPaymentTransaction(row), nil
}

func (r *PaymentRepository) MarkStatus(ctx context.Context, transactionID, status string, paymentMethod int64, paidAt *time.Time) error {
	rawPaidAt := sql.NullString{}
	if paidAt != nil {
		rawPaidAt = sql.NullString{
			String: paidAt.UTC().Format(time.RFC3339Nano),
			Valid:  true,
		}
	}
	return r.q.MarkPaymentTransactionStatus(ctx, sqlc.MarkPaymentTransactionStatusParams{
		Status:        status,
		PaymentMethod: paymentMethod,
		PaidAt:        rawPaidAt,
		UpdatedAt:     nowUTC(),
		TransactionID: transactionID,
	})
}

func (r *PaymentRepository) MarkRewarded(ctx context.Context, transactionID string) error {
	return r.q.MarkPaymentTransactionRewarded(ctx, sqlc.MarkPaymentTransactionRewardedParams{
		UpdatedAt:     nowUTC(),
		TransactionID: transactionID,
	})
}

func mapPaymentTransaction(row sqlc.PaymentTransaction) PaymentTransaction {
	item := PaymentTransaction{
		ID:             row.ID,
		TransactionID:  row.TransactionID,
		UserID:         row.UserID,
		PlatformUserID: row.PlatformUserID,
		ProductKey:     row.ProductKey,
		ProductTitle:   row.ProductTitle,
		Coins:          row.Coins,
		Amount:         row.Amount,
		Currency:       row.Currency,
		PaymentMethod:  row.PaymentMethod,
		Status:         row.Status,
		RedirectURL:    row.RedirectUrl,
		Rewarded:       row.Rewarded,
		CreatedAt:      parseTime(row.CreatedAt),
		UpdatedAt:      parseTime(row.UpdatedAt),
	}
	if row.PaidAt.Valid {
		t := parseTime(row.PaidAt.String)
		item.PaidAt = &t
	}
	return item
}
