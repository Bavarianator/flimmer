-- Sammlungen (kind 'collection', für alle, user_id NULL) und Wiedergabelisten (kind 'playlist', pro Profil).
-- items ist ein JSON-Array aus Titel-IDs oder "serie:<Name>" in Anzeigereihenfolge.
CREATE TABLE IF NOT EXISTS lists (
	id         TEXT PRIMARY KEY,
	kind       TEXT NOT NULL,
	user_id    TEXT REFERENCES users(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	overview   TEXT NOT NULL DEFAULT '',
	items      TEXT NOT NULL DEFAULT '[]',
	created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS lists_owner ON lists(kind, user_id);
