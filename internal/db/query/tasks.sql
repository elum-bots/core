-- name: CreateTask :one
INSERT INTO tasks (
  reward,
  active,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?)
RETURNING id, reward, active, created_at, updated_at;

-- name: GetTask :one
SELECT
  id,
  reward,
  active,
  created_at,
  updated_at
FROM tasks
WHERE id = ?
LIMIT 1;

-- name: UpdateTask :execrows
UPDATE tasks
SET
  reward = ?,
  active = ?,
  updated_at = ?
WHERE id = ?;

-- name: SetTaskActive :execrows
UPDATE tasks
SET
  active = ?,
  updated_at = ?
WHERE id = ?;

-- name: ListTasksWithStats :many
SELECT
  t.id,
  t.reward,
  t.active,
  t.created_at,
  t.updated_at,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
  ), 0) AS INTEGER) AS completed_total,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
      AND tr.rewarded_at >= ?
      AND tr.rewarded_at < ?
  ), 0) AS INTEGER) AS completed_today,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
      AND tr.rewarded_at >= ?
      AND tr.rewarded_at < ?
  ), 0) AS INTEGER) AS completed_yesterday
FROM tasks t
WHERE t.active = TRUE
ORDER BY t.id ASC;

-- name: GetTaskWithStats :one
SELECT
  t.id,
  t.reward,
  t.active,
  t.created_at,
  t.updated_at,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
  ), 0) AS INTEGER) AS completed_total,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
      AND tr.rewarded_at >= ?
      AND tr.rewarded_at < ?
  ), 0) AS INTEGER) AS completed_today,
  CAST(COALESCE((
    SELECT COUNT(*)
    FROM task_rewards tr
    WHERE tr.task_id = t.id
      AND tr.rewarded_at >= ?
      AND tr.rewarded_at < ?
  ), 0) AS INTEGER) AS completed_yesterday
FROM tasks t
WHERE t.id = ?
LIMIT 1;

-- name: DeleteTaskChannelsByTaskID :exec
DELETE FROM task_channels
WHERE task_id = ?;

-- name: InsertTaskChannel :exec
INSERT INTO task_channels (
  task_id,
  channel_id,
  title,
  url,
  requires_check,
  sort_order,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListTaskChannelsByTaskID :many
SELECT
  id,
  task_id,
  channel_id,
  title,
  url,
  requires_check,
  sort_order,
  created_at,
  updated_at
FROM task_channels
WHERE task_id = ?
ORDER BY sort_order ASC, id ASC;

-- name: CountTaskRewardForUser :one
SELECT COUNT(*)
FROM task_rewards
WHERE task_id = ?
  AND user_id = ?;

-- name: InsertTaskReward :execrows
INSERT OR IGNORE INTO task_rewards (
  task_id,
  user_id,
  reward,
  rewarded_at
) VALUES (?, ?, ?, ?);
