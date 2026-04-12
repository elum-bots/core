-- name: StartBroadcastStats :one
INSERT INTO broadcast_stats (
  date,
  type,
  total,
  success,
  error,
  active,
  status,
  stop_requested,
  admin_chat_id,
  payload_json,
  created_at,
  updated_at
) VALUES (?, ?, ?, 0, 0, TRUE, 'running', FALSE, ?, ?, ?, ?)
RETURNING id, date, type, total, success, error, active, status, stop_requested, admin_chat_id, payload_json, created_at, updated_at, finished_at;

-- name: IncBroadcastSuccess :exec
UPDATE broadcast_stats
SET
  success = success + 1,
  updated_at = ?
WHERE id = ?;

-- name: IncBroadcastError :exec
UPDATE broadcast_stats
SET
  error = error + 1,
  updated_at = ?
WHERE id = ?;

-- name: RequestStopBroadcastStats :execrows
UPDATE broadcast_stats
SET
  status = 'cancel_requested',
  stop_requested = TRUE,
  updated_at = ?
WHERE id = ?
  AND active = TRUE;

-- name: FinishBroadcastStats :exec
UPDATE broadcast_stats
SET
  active = FALSE,
  status = ?,
  updated_at = ?,
  finished_at = ?
WHERE id = ?;

-- name: GetBroadcastStats :one
SELECT
  id,
  date,
  type,
  total,
  success,
  error,
  active,
  status,
  stop_requested,
  admin_chat_id,
  payload_json,
  created_at,
  updated_at,
  finished_at
FROM broadcast_stats
WHERE id = ?
LIMIT 1;

-- name: CountBroadcastStatsTotal :one
SELECT COUNT(*)
FROM broadcast_stats;

-- name: CountBroadcastStatsActive :one
SELECT COUNT(*)
FROM broadcast_stats
WHERE active = TRUE;

-- name: ListBroadcastStats :many
SELECT
  id,
  date,
  type,
  total,
  success,
  error,
  active,
  status,
  stop_requested,
  admin_chat_id,
  payload_json,
  created_at,
  updated_at,
  finished_at
FROM broadcast_stats
ORDER BY created_at DESC
LIMIT ?;

-- name: ListActiveBroadcastStats :many
SELECT
  id,
  date,
  type,
  total,
  success,
  error,
  active,
  status,
  stop_requested,
  admin_chat_id,
  payload_json,
  created_at,
  updated_at,
  finished_at
FROM broadcast_stats
WHERE active = TRUE
ORDER BY created_at DESC;

-- name: ListResumableBroadcastStats :many
SELECT
  id,
  date,
  type,
  total,
  success,
  error,
  active,
  status,
  stop_requested,
  admin_chat_id,
  payload_json,
  created_at,
  updated_at,
  finished_at
FROM broadcast_stats
WHERE active = TRUE
ORDER BY created_at ASC;

-- name: CreateBroadcastTarget :exec
INSERT INTO broadcast_targets (
  broadcast_id,
  user_id,
  sort_order,
  status,
  error,
  updated_at
) VALUES (?, ?, ?, 'pending', '', ?)
ON CONFLICT(broadcast_id, user_id) DO NOTHING;

-- name: ListPendingBroadcastTargets :many
SELECT
  broadcast_id,
  user_id,
  sort_order,
  status,
  error,
  updated_at
FROM broadcast_targets
WHERE broadcast_id = ?
  AND status = 'pending'
ORDER BY sort_order
LIMIT ?;

-- name: MarkBroadcastTargetSent :execrows
UPDATE broadcast_targets
SET
  status = 'sent',
  error = '',
  updated_at = ?
WHERE broadcast_id = ?
  AND user_id = ?
  AND status = 'pending';

-- name: MarkBroadcastTargetError :execrows
UPDATE broadcast_targets
SET
  status = 'error',
  error = ?,
  updated_at = ?
WHERE broadcast_id = ?
  AND user_id = ?
  AND status = 'pending';
