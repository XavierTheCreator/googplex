-- Video length and live state from videos.list, shown on thumbnails.
ALTER TABLE yt_videos ADD COLUMN duration_seconds INTEGER NOT NULL DEFAULT 0;
ALTER TABLE yt_videos ADD COLUMN live TEXT NOT NULL DEFAULT ''; -- '', 'live' or 'upcoming'
