-- name: CreateMetricEvent :exec
INSERT INTO metric_events (
  kind,
  user_id,
  ref_id,
  value,
  created_at
) VALUES (?, ?, ?, ?, ?);

-- name: CountMetricEventsTotal :one
SELECT COUNT(*)
FROM metric_events
WHERE kind = ?;

-- name: CountMetricEventsBetween :one
SELECT COUNT(*)
FROM metric_events
WHERE kind = ?
  AND created_at >= ?
  AND created_at < ?;

-- name: CountUsersTotal :one
SELECT COUNT(*)
FROM users;

-- name: CountUsersCreatedBetween :one
SELECT COUNT(*)
FROM users
WHERE created_at >= ?
  AND created_at < ?;

-- name: CountUsersUpdatedBetween :one
SELECT COUNT(*)
FROM users
WHERE updated_at >= ?
  AND updated_at < ?;

-- name: CountReferredUsersTotal :one
SELECT COUNT(*)
FROM users
WHERE referral_by != '';

-- name: CountReferredUsersBetween :one
SELECT COUNT(*)
FROM users
WHERE referral_by != ''
  AND created_at >= ?
  AND created_at < ?;
