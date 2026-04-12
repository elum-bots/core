-- name: CreatePost :one
INSERT INTO posts (
  title,
  text,
  media_id,
  media_kind,
  buttons_json,
  created_by,
  created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, title, text, media_id, media_kind, buttons_json, created_by, created_at;

-- name: ListPosts :many
SELECT id, title, text, media_id, media_kind, buttons_json, created_by, created_at
FROM posts
ORDER BY id DESC;

-- name: GetPost :one
SELECT id, title, text, media_id, media_kind, buttons_json, created_by, created_at
FROM posts
WHERE id = ?
LIMIT 1;

-- name: SavePostDelivery :exec
INSERT INTO post_deliveries (
  post_id,
  user_id,
  status,
  error,
  sent_at
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(post_id, user_id) DO UPDATE SET
  status = excluded.status,
  error = excluded.error,
  sent_at = excluded.sent_at;
