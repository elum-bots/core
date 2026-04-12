-- name: GetUser :one
SELECT
  user_id,
  name,
  birth_date,
  theme,
  coins,
  detailed,
  daily_card_date,
  daily_card_streak,
  daily_reminder_date,
  referral_by,
  referral_cnt,
  referral_reward_progress,
  created_at,
  updated_at
FROM users
WHERE user_id = ?
LIMIT 1;

-- name: ListUsers :many
SELECT
  user_id,
  name,
  birth_date,
  theme,
  coins,
  detailed,
  daily_card_date,
  daily_card_streak,
  daily_reminder_date,
  referral_by,
  referral_cnt,
  referral_reward_progress,
  created_at,
  updated_at
FROM users
ORDER BY created_at DESC;

-- name: ListUserIDs :many
SELECT user_id
FROM users
ORDER BY created_at;

-- name: CreateUserIfMissing :exec
INSERT INTO users (
  user_id,
  coins,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO NOTHING;

-- name: UpdateUserProfile :exec
UPDATE users
SET
  name = ?,
  birth_date = ?,
  updated_at = ?
WHERE user_id = ?;

-- name: TouchUser :exec
UPDATE users
SET updated_at = ?
WHERE user_id = ?;

-- name: UpdateUserTheme :exec
UPDATE users
SET
  theme = ?,
  updated_at = ?
WHERE user_id = ?;

-- name: AddUserCoins :exec
UPDATE users
SET
  coins = coins + ?,
  updated_at = ?
WHERE user_id = ?;

-- name: SpendUserCoins :execrows
UPDATE users
SET
  coins = coins - ?,
  updated_at = ?
WHERE user_id = ?
  AND coins >= ?;

-- name: RegisterUserReferral :execrows
UPDATE users
SET
  referral_by = ?,
  updated_at = ?
WHERE user_id = ?
  AND referral_by = ''
  AND user_id <> ?;

-- name: IncrementUserReferralCount :exec
UPDATE users
SET
  referral_cnt = referral_cnt + 1,
  updated_at = ?
WHERE user_id = ?;

-- name: UpdateUserReferralRewardProgress :exec
UPDATE users
SET
  referral_reward_progress = ?,
  updated_at = ?
WHERE user_id = ?;
