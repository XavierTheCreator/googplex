CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

-- One row per signed-in Google account. Secrets live in the OS keychain, never here.
CREATE TABLE accounts (
	id           INTEGER PRIMARY KEY,
	email        TEXT NOT NULL UNIQUE,
	name         TEXT NOT NULL DEFAULT '',
	picture      TEXT NOT NULL DEFAULT '',
	scopes       TEXT NOT NULL DEFAULT '',  -- space separated, as Google returns them
	token_expiry TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL DEFAULT 'ok', -- ok | needs_reconnect
	created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Per-service sync bookkeeping (Gmail historyId, Calendar syncToken, ...).
CREATE TABLE sync_state (
	service    TEXT NOT NULL,
	account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	cursor     TEXT NOT NULL DEFAULT '',
	last_run   TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (service, account_id)
);
