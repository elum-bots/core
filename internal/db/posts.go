package db

import (
	"context"
	"encoding/json"

	"github.com/elum-bots/core/internal/db/sqlc"
)

type PostRepository struct {
	q *sqlc.Queries
}

func NewPostRepository(q *sqlc.Queries) *PostRepository {
	return &PostRepository{q: q}
}

func (r *PostRepository) Create(ctx context.Context, title, text, mediaID, mediaKind string, buttons []PostButton, createdBy string) (Post, error) {
	raw, err := json.Marshal(buttons)
	if err != nil {
		return Post{}, err
	}
	row, err := r.q.CreatePost(ctx, sqlc.CreatePostParams{
		Title:       title,
		Text:        text,
		MediaID:     mediaID,
		MediaKind:   mediaKind,
		ButtonsJson: string(raw),
		CreatedBy:   createdBy,
		CreatedAt:   nowUTC(),
	})
	if err != nil {
		return Post{}, err
	}
	return mapPost(row.ID, row.Title, row.Text, row.MediaID, row.MediaKind, row.ButtonsJson, row.CreatedBy, row.CreatedAt)
}

func (r *PostRepository) List(ctx context.Context) ([]Post, error) {
	rows, err := r.q.ListPosts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Post, 0, len(rows))
	for _, row := range rows {
		item, err := mapPost(row.ID, row.Title, row.Text, row.MediaID, row.MediaKind, row.ButtonsJson, row.CreatedBy, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *PostRepository) Get(ctx context.Context, id int64) (Post, error) {
	row, err := r.q.GetPost(ctx, id)
	if err != nil {
		return Post{}, err
	}
	return mapPost(row.ID, row.Title, row.Text, row.MediaID, row.MediaKind, row.ButtonsJson, row.CreatedBy, row.CreatedAt)
}

func (r *PostRepository) SaveDelivery(ctx context.Context, postID int64, userID, status, errText string) error {
	return r.q.SavePostDelivery(ctx, sqlc.SavePostDeliveryParams{
		PostID: postID,
		UserID: userID,
		Status: status,
		Error:  errText,
		SentAt: nowUTC(),
	})
}

func mapPost(id int64, title, text, mediaID, mediaKind, buttonsJSON, createdBy, createdAt string) (Post, error) {
	item := Post{
		ID:        id,
		Title:     title,
		Text:      text,
		MediaID:   mediaID,
		MediaKind: mediaKind,
		CreatedBy: createdBy,
		CreatedAt: parseTime(createdAt),
	}
	if buttonsJSON == "" {
		return item, nil
	}
	if err := json.Unmarshal([]byte(buttonsJSON), &item.Buttons); err != nil {
		return Post{}, err
	}
	return item, nil
}
