CREATE TABLE IF NOT EXISTS users (
  user_id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  birth_date TEXT NOT NULL DEFAULT '',
  theme TEXT NOT NULL DEFAULT '',
  coins INTEGER NOT NULL DEFAULT 0,
  detailed BOOLEAN NOT NULL DEFAULT FALSE,
  daily_card_date TEXT NOT NULL DEFAULT '',
  daily_card_streak INTEGER NOT NULL DEFAULT 0,
  daily_reminder_date TEXT NOT NULL DEFAULT '',
  referral_by TEXT NOT NULL DEFAULT '',
  referral_cnt INTEGER NOT NULL DEFAULT 0,
  referral_reward_progress REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS mandatory_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  requires_check BOOLEAN NOT NULL DEFAULT TRUE,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_mandatory_status (
  user_id TEXT NOT NULL,
  channel_row_id INTEGER NOT NULL,
  subscribed BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, channel_row_id),
  FOREIGN KEY (channel_row_id) REFERENCES mandatory_channels(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS mandatory_reward_progress (
  channel_row_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  processed_at TEXT NOT NULL,
  PRIMARY KEY (channel_row_id, user_id),
  FOREIGN KEY (channel_row_id) REFERENCES mandatory_channels(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  media_id TEXT NOT NULL DEFAULT '',
  media_kind TEXT NOT NULL DEFAULT '',
  buttons_json TEXT NOT NULL DEFAULT '[]',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS post_deliveries (
  post_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  sent_at TEXT NOT NULL,
  PRIMARY KEY (post_id, user_id),
  FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS payment_transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  transaction_id TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL,
  platform_user_id TEXT NOT NULL DEFAULT '',
  product_key TEXT NOT NULL,
  product_title TEXT NOT NULL,
  coins INTEGER NOT NULL DEFAULT 0,
  amount INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'RUB',
  payment_method INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  redirect_url TEXT NOT NULL DEFAULT '',
  rewarded BOOLEAN NOT NULL DEFAULT FALSE,
  paid_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS integration_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  provider TEXT NOT NULL,
  token TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(provider, token)
);

CREATE TABLE IF NOT EXISTS broadcast_stats (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  date TEXT NOT NULL,
  type TEXT NOT NULL,
  total INTEGER NOT NULL DEFAULT 0,
  success INTEGER NOT NULL DEFAULT 0,
  error INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  status TEXT NOT NULL DEFAULT 'running',
  stop_requested BOOLEAN NOT NULL DEFAULT FALSE,
  admin_chat_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  finished_at TEXT
);

CREATE TABLE IF NOT EXISTS broadcast_targets (
  broadcast_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (broadcast_id, user_id),
  FOREIGN KEY (broadcast_id) REFERENCES broadcast_stats(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS track_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS track_link_stats (
  link_id INTEGER PRIMARY KEY,
  arrivals_count INTEGER NOT NULL DEFAULT 0,
  generated_users_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (link_id) REFERENCES track_links(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS track_link_visits (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL UNIQUE,
  link_id INTEGER NOT NULL,
  visited_at TEXT NOT NULL,
  FOREIGN KEY (link_id) REFERENCES track_links(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS track_link_generated_users (
  user_id TEXT PRIMARY KEY,
  link_id INTEGER NOT NULL,
  generated_at TEXT NOT NULL,
  FOREIGN KEY (link_id) REFERENCES track_links(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS metric_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  user_id TEXT NOT NULL DEFAULT '',
  ref_id INTEGER NOT NULL DEFAULT 0,
  value INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reward INTEGER NOT NULL DEFAULT 1,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL,
  channel_id TEXT NOT NULL,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  requires_check BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_rewards (
  task_id INTEGER NOT NULL,
  user_id TEXT NOT NULL,
  reward INTEGER NOT NULL DEFAULT 0,
  rewarded_at TEXT NOT NULL,
  PRIMARY KEY (task_id, user_id),
  FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE INDEX IF NOT EXISTS idx_users_referral_by ON users(referral_by);
CREATE INDEX IF NOT EXISTS idx_mandatory_channels_active ON mandatory_channels(active, id);
CREATE INDEX IF NOT EXISTS idx_user_mandatory_status_channel_row_id ON user_mandatory_status(channel_row_id, subscribed);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_user_id ON payment_transactions(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_status ON payment_transactions(status, created_at);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_transaction_id ON payment_transactions(transaction_id);
CREATE INDEX IF NOT EXISTS idx_integration_tokens_provider_id ON integration_tokens(provider, id);
CREATE INDEX IF NOT EXISTS idx_broadcast_stats_active ON broadcast_stats(active, date);
CREATE INDEX IF NOT EXISTS idx_broadcast_stats_status ON broadcast_stats(status, created_at);
CREATE INDEX IF NOT EXISTS idx_broadcast_targets_status ON broadcast_targets(broadcast_id, status, sort_order);
CREATE INDEX IF NOT EXISTS idx_track_links_code ON track_links(code);
CREATE INDEX IF NOT EXISTS idx_track_link_visits_link_id ON track_link_visits(link_id);
CREATE INDEX IF NOT EXISTS idx_track_link_generated_users_link_id ON track_link_generated_users(link_id);
CREATE INDEX IF NOT EXISTS idx_metric_events_kind_created_at ON metric_events(kind, created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_active_id ON tasks(active, id);
CREATE INDEX IF NOT EXISTS idx_task_channels_task_id_sort ON task_channels(task_id, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_task_rewards_user_id ON task_rewards(user_id, rewarded_at);
CREATE INDEX IF NOT EXISTS idx_task_rewards_task_id ON task_rewards(task_id, rewarded_at);
