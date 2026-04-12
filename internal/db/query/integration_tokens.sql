-- name: CreateIntegrationToken :one
INSERT INTO integration_tokens (
  provider,
  token,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?)
RETURNING id, provider, token, created_at, updated_at;

-- name: ListIntegrationTokensByProvider :many
SELECT
  id,
  provider,
  token,
  created_at,
  updated_at
FROM integration_tokens
WHERE provider = ?
ORDER BY id DESC;

-- name: GetIntegrationTokenByID :one
SELECT
  id,
  provider,
  token,
  created_at,
  updated_at
FROM integration_tokens
WHERE id = ?
LIMIT 1;

-- name: UpdateIntegrationToken :execrows
UPDATE integration_tokens
SET
  token = ?,
  updated_at = ?
WHERE id = ?
  AND provider = ?;

-- name: DeleteIntegrationToken :execrows
DELETE FROM integration_tokens
WHERE id = ?
  AND provider = ?;
