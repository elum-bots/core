package db

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/elum-bots/core/internal/db/sqlc"
	"github.com/elum-utils/env"
)

type UserRepository struct {
	store *Store
	q     *sqlc.Queries
}

func NewUserRepository(store *Store, q *sqlc.Queries) *UserRepository {
	return &UserRepository{store: store, q: q}
}

func (r *UserRepository) Ensure(ctx context.Context, userID string) (sqlc.User, error) {
	now := nowUTC()
	if err := r.q.CreateUserIfMissing(ctx, sqlc.CreateUserIfMissingParams{
		UserID:    userID,
		Coins:     int64(env.GetEnvInt("ONBOARDING_BONUS", 1)),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return sqlc.User{}, err
	}
	return r.q.GetUser(ctx, userID)
}

func (r *UserRepository) Get(ctx context.Context, userID string) (sqlc.User, error) {
	return r.q.GetUser(ctx, userID)
}

func (r *UserRepository) List(ctx context.Context) ([]sqlc.User, error) {
	return r.q.ListUsers(ctx)
}

func (r *UserRepository) ListUserIDs(ctx context.Context) ([]string, error) {
	return r.q.ListUserIDs(ctx)
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID, name, birthDate string) error {
	return r.q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		Name:      name,
		BirthDate: birthDate,
		UpdatedAt: nowUTC(),
		UserID:    userID,
	})
}

func (r *UserRepository) Touch(ctx context.Context, userID string) error {
	return r.q.TouchUser(ctx, sqlc.TouchUserParams{
		UpdatedAt: nowUTC(),
		UserID:    userID,
	})
}

func (r *UserRepository) UpdateTheme(ctx context.Context, userID, theme string) error {
	return r.q.UpdateUserTheme(ctx, sqlc.UpdateUserThemeParams{
		Theme:     theme,
		UpdatedAt: nowUTC(),
		UserID:    userID,
	})
}

func (r *UserRepository) AddCoins(ctx context.Context, userID string, amount int64) error {
	return r.q.AddUserCoins(ctx, sqlc.AddUserCoinsParams{
		Coins:     amount,
		UpdatedAt: nowUTC(),
		UserID:    userID,
	})
}

func (r *UserRepository) SpendCoins(ctx context.Context, userID string, amount int64) (bool, error) {
	rows, err := r.q.SpendUserCoins(ctx, sqlc.SpendUserCoinsParams{
		Coins:     amount,
		UpdatedAt: nowUTC(),
		UserID:    userID,
		Coins_2:   amount,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *UserRepository) RegisterReferral(ctx context.Context, invitedUserID, referrerUserID string) (bool, error) {
	linked, _, _, err := r.RegisterReferralWithReward(ctx, invitedUserID, referrerUserID, 0)
	return linked, err
}

func (r *UserRepository) RegisterReferralWithReward(ctx context.Context, invitedUserID, referrerUserID string, rewardPerReferral float64) (bool, int64, float64, error) {
	if invitedUserID == "" || referrerUserID == "" || invitedUserID == referrerUserID {
		return false, 0, 0, nil
	}

	var (
		linked    bool
		granted   int64
		remainder float64
	)
	err := r.store.WithTx(ctx, func(q *sqlc.Queries) error {
		now := nowUTC()

		affected, err := q.RegisterUserReferral(ctx, sqlc.RegisterUserReferralParams{
			ReferralBy: referrerUserID,
			UpdatedAt:  now,
			UserID:     invitedUserID,
			UserID_2:   referrerUserID,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		linked = true

		if err := q.IncrementUserReferralCount(ctx, sqlc.IncrementUserReferralCountParams{
			UpdatedAt: now,
			UserID:    referrerUserID,
		}); err != nil {
			return err
		}

		if rewardPerReferral <= 0 {
			return nil
		}

		referrer, err := q.GetUser(ctx, referrerUserID)
		if err != nil {
			return err
		}

		granted, remainder = splitReferralReward(referrer.ReferralRewardProgress + rewardPerReferral)
		if err := q.UpdateUserReferralRewardProgress(ctx, sqlc.UpdateUserReferralRewardProgressParams{
			ReferralRewardProgress: remainder,
			UpdatedAt:              nowUTC(),
			UserID:                 referrerUserID,
		}); err != nil {
			return err
		}

		if granted > 0 {
			if err := q.AddUserCoins(ctx, sqlc.AddUserCoinsParams{
				Coins:     granted,
				UpdatedAt: nowUTC(),
				UserID:    referrerUserID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, 0, 0, err
	}
	return linked, granted, remainder, nil
}

func (r *UserRepository) MustGet(ctx context.Context, userID string) (sqlc.User, error) {
	u, err := r.q.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, err
		}
		return sqlc.User{}, err
	}
	return u, nil
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func splitReferralReward(progress float64) (int64, float64) {
	const eps = 0.00001

	if progress <= 0 {
		return 0, 0
	}

	granted := int64(math.Floor(progress + eps))
	remainder := progress - float64(granted)

	if math.Abs(remainder) < eps {
		remainder = 0
	}
	if remainder < 0 {
		remainder = 0
	}
	if remainder >= 1 {
		extra := int64(math.Floor(remainder + eps))
		granted += extra
		remainder -= float64(extra)
	}

	return granted, remainder
}
