-- YouTube no longer exposes Watch later or watch history in its API, so the
-- app keeps its own: a Watch later list and a history of what played here.
CREATE TABLE yt_watch_later (
	video_id TEXT PRIMARY KEY,
	data     TEXT NOT NULL,    -- the youtube.Video as JSON
	added_at INTEGER NOT NULL  -- unix ms
);
CREATE TABLE yt_history (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	video_id  TEXT NOT NULL,
	data      TEXT NOT NULL,
	played_at INTEGER NOT NULL
);
CREATE INDEX yt_history_played ON yt_history(played_at DESC);
