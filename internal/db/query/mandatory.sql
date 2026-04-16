-- name: CreateMandatoryChannel :one
INSERT INTO mandatory_channels (
  channel_id,
  title,
  url,
  requires_check,
  active,
  created_at
) VALUES (?, ?, ?, ?, TRUE, ?)
RETURNING id, channel_id, title, url, requires_check, active, created_at;

-- name: ListMandatoryChannels :many
SELECT id, channel_id, title, url, requires_check, active, created_at
FROM mandatory_channels
WHERE active = TRUE
ORDER BY id;

-- name: DeleteMandatoryChannel :exec
DELETE FROM mandatory_channels
WHERE id = ?;

-- name: UpsertMandatorySubscription :exec
INSERT INTO user_mandatory_status (
  user_id,
  channel_row_id,
  subscribed,
  updated_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(user_id, channel_row_id) DO UPDATE SET
  subscribed = excluded.subscribed,
  updated_at = excluded.updated_at;

-- name: ListUsersPendingMandatoryReward :many
SELECT u.user_id
FROM users u
LEFT JOIN user_mandatory_status s
  ON s.user_id = u.user_id
 AND s.channel_row_id = ?
LEFT JOIN mandatory_reward_progress p
  ON p.channel_row_id = ?
 AND p.user_id = u.user_id
WHERE COALESCE(s.subscribed, FALSE) = FALSE
  AND p.user_id IS NULL
ORDER BY u.user_id;

-- name: GetMandatoryChannel :one
SELECT id, channel_id, title, url, requires_check, active, created_at
FROM mandatory_channels
WHERE id = ?
LIMIT 1;

-- name: ClaimMandatoryRewardProgress :execrows
INSERT INTO mandatory_reward_progress (
  channel_row_id,
  user_id,
  processed_at
) VALUES (?, ?, ?)
ON CONFLICT(channel_row_id, user_id) DO NOTHING;

-- name: ReleaseMandatoryRewardProgress :exec
DELETE FROM mandatory_reward_progress
WHERE channel_row_id = ?
  AND user_id = ?;

-- name: ResetMandatoryRewardProgress :exec
DELETE FROM mandatory_reward_progress
WHERE channel_row_id = ?;

-- name: CountMandatoryVerifiedUsersTotal :one
SELECT COUNT(DISTINCT user_id) AS verified_count
FROM user_mandatory_status
WHERE subscribed = TRUE;

-- name: CountMandatoryVerifiedUsersBetween :one
SELECT COUNT(DISTINCT user_id) AS verified_count
FROM user_mandatory_status
WHERE subscribed = TRUE
  AND updated_at >= ?
  AND updated_at < ?;
