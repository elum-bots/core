package db

import (
	"context"
	"strings"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type IntegrationTokenRepository struct {
	q *sqlc.Queries
}

func NewIntegrationTokenRepository(q *sqlc.Queries) *IntegrationTokenRepository {
	return &IntegrationTokenRepository{q: q}
}

func (r *IntegrationTokenRepository) Create(ctx context.Context, provider, token string) (IntegrationToken, error) {
	row, err := r.q.CreateIntegrationToken(ctx, sqlc.CreateIntegrationTokenParams{
		Provider:  normalizeIntegrationProvider(provider),
		Token:     strings.TrimSpace(token),
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	})
	if err != nil {
		return IntegrationToken{}, err
	}
	return mapIntegrationToken(row.ID, row.Provider, row.Token, row.CreatedAt, row.UpdatedAt), nil
}

func (r *IntegrationTokenRepository) ListByProvider(ctx context.Context, provider string) ([]IntegrationToken, error) {
	rows, err := r.q.ListIntegrationTokensByProvider(ctx, normalizeIntegrationProvider(provider))
	if err != nil {
		return nil, err
	}
	out := make([]IntegrationToken, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapIntegrationToken(row.ID, row.Provider, row.Token, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

func (r *IntegrationTokenRepository) ValuesByProvider(ctx context.Context, provider string) ([]string, error) {
	items, err := r.ListByProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		token := strings.TrimSpace(item.Token)
		if token != "" {
			out = append(out, token)
		}
	}
	return out, nil
}

func (r *IntegrationTokenRepository) Get(ctx context.Context, id int64) (IntegrationToken, error) {
	row, err := r.q.GetIntegrationTokenByID(ctx, id)
	if err != nil {
		return IntegrationToken{}, err
	}
	return mapIntegrationToken(row.ID, row.Provider, row.Token, row.CreatedAt, row.UpdatedAt), nil
}

func (r *IntegrationTokenRepository) Update(ctx context.Context, provider string, id int64, token string) (bool, error) {
	affected, err := r.q.UpdateIntegrationToken(ctx, sqlc.UpdateIntegrationTokenParams{
		Token:     strings.TrimSpace(token),
		UpdatedAt: nowUTC(),
		ID:        id,
		Provider:  normalizeIntegrationProvider(provider),
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *IntegrationTokenRepository) Delete(ctx context.Context, provider string, id int64) (bool, error) {
	affected, err := r.q.DeleteIntegrationToken(ctx, sqlc.DeleteIntegrationTokenParams{
		ID:       id,
		Provider: normalizeIntegrationProvider(provider),
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func normalizeIntegrationProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func mapIntegrationToken(id int64, provider, token, createdAt, updatedAt string) IntegrationToken {
	return IntegrationToken{
		ID:        id,
		Provider:  normalizeIntegrationProvider(provider),
		Token:     strings.TrimSpace(token),
		CreatedAt: parseTime(createdAt),
		UpdatedAt: parseTime(updatedAt),
	}
}
