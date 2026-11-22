-- Inbox threads as last seen from Gmail, so the list shows instantly on open.
CREATE TABLE gmail_threads (
	id            TEXT PRIMARY KEY,
	subject       TEXT NOT NULL DEFAULT '',
	from_name     TEXT NOT NULL DEFAULT '',
	from_email    TEXT NOT NULL DEFAULT '',
	snippet       TEXT NOT NULL DEFAULT '',
	date          INTEGER NOT NULL DEFAULT 0, -- unix ms of the newest message
	unread        INTEGER NOT NULL DEFAULT 0,
	message_count INTEGER NOT NULL DEFAULT 1,
	in_inbox      INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX gmail_threads_date ON gmail_threads(date DESC);

-- Latest uploads from subscribed channels, from their public RSS feeds.
CREATE TABLE yt_videos (
	id            TEXT PRIMARY KEY,
	channel_id    TEXT NOT NULL,
	channel_title TEXT NOT NULL DEFAULT '',
	title         TEXT NOT NULL DEFAULT '',
	description   TEXT NOT NULL DEFAULT '',
	thumbnail     TEXT NOT NULL DEFAULT '',
	published_at  TEXT NOT NULL DEFAULT '', -- RFC 3339
	views         INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX yt_videos_published ON yt_videos(published_at DESC);
