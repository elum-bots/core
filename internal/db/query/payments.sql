-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
  transaction_id,
  user_id,
  platform_user_id,
  product_key,
  product_title,
  coins,
  amount,
  currency,
  payment_method,
  status,
  redirect_url,
  rewarded,
  paid_at,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, transaction_id, user_id, platform_user_id, product_key, product_title, coins, amount, currency, payment_method, status, redirect_url, rewarded, paid_at, created_at, updated_at;

-- name: GetPaymentTransactionByID :one
SELECT
  id,
  transaction_id,
  user_id,
  platform_user_id,
  product_key,
  product_title,
  coins,
  amount,
  currency,
  payment_method,
  status,
  redirect_url,
  rewarded,
  paid_at,
  created_at,
  updated_at
FROM payment_transactions
WHERE transaction_id = ?
LIMIT 1;

-- name: MarkPaymentTransactionStatus :exec
UPDATE payment_transactions
SET
  status = ?,
  payment_method = ?,
  paid_at = ?,
  updated_at = ?
WHERE transaction_id = ?;

-- name: MarkPaymentTransactionRewarded :exec
UPDATE payment_transactions
SET
  rewarded = TRUE,
  updated_at = ?
WHERE transaction_id = ?;
