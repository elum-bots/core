-- name: CreateTrackLink :one
INSERT INTO track_links (
  code,
  label,
  created_by_user_id,
  created_at
) VALUES (?, ?, ?, ?)
RETURNING id, code, label, created_by_user_id, created_at;

-- name: CreateTrackLinkStats :exec
INSERT INTO track_link_stats (
  link_id,
  arrivals_count,
  generated_users_count,
  updated_at
) VALUES (?, 0, 0, ?);

-- name: ListTrackLinks :many
SELECT
  l.id,
  l.code,
  l.label,
  l.created_by_user_id,
  l.created_at,
  COALESCE(s.arrivals_count, 0) AS arrivals_count,
  COALESCE(s.generated_users_count, 0) AS generated_users_count
FROM track_links l
LEFT JOIN track_link_stats s ON s.link_id = l.id
ORDER BY l.id DESC;

-- name: GetTrackLink :one
SELECT
  l.id,
  l.code,
  l.label,
  l.created_by_user_id,
  l.created_at,
  COALESCE(s.arrivals_count, 0) AS arrivals_count,
  COALESCE(s.generated_users_count, 0) AS generated_users_count
FROM track_links l
LEFT JOIN track_link_stats s ON s.link_id = l.id
WHERE l.id = ?
LIMIT 1;

-- name: GetTrackLinkByCode :one
SELECT
  l.id,
  l.code,
  l.label,
  l.created_by_user_id,
  l.created_at,
  COALESCE(s.arrivals_count, 0) AS arrivals_count,
  COALESCE(s.generated_users_count, 0) AS generated_users_count
FROM track_links l
LEFT JOIN track_link_stats s ON s.link_id = l.id
WHERE l.code = ?
LIMIT 1;

-- name: DeleteTrackLink :exec
DELETE FROM track_links
WHERE id = ?;

-- name: InsertTrackVisit :execrows
INSERT OR IGNORE INTO track_link_visits (
  user_id,
  link_id,
  visited_at
) VALUES (?, ?, ?);

-- name: IncrementTrackArrivals :exec
UPDATE track_link_stats
SET
  arrivals_count = arrivals_count + 1,
  updated_at = ?
WHERE link_id = ?;

-- name: InsertTrackGeneratedUser :execrows
INSERT OR IGNORE INTO track_link_generated_users (
  user_id,
  link_id,
  generated_at
) VALUES (?, ?, ?);

-- name: IncrementTrackGeneratedUsers :exec
UPDATE track_link_stats
SET
  generated_users_count = generated_users_count + 1,
  updated_at = ?
WHERE link_id = ?;

-- name: CountTrackVisitsTotal :one
SELECT COUNT(*)
FROM track_link_visits;

-- name: CountTrackVisitsBetween :one
SELECT COUNT(*)
FROM track_link_visits
WHERE visited_at >= ?
  AND visited_at < ?;
