-- Zeiten sind Unix-Millisekunden (INTEGER), Wahrheitswerte 0/1.

CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) STRICT;

CREATE TABLE libraries (
	id   INTEGER PRIMARY KEY,
	path TEXT NOT NULL UNIQUE
) STRICT;

CREATE TABLE users (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	color      INTEGER NOT NULL DEFAULT 0,
	admin      INTEGER NOT NULL DEFAULT 0,
	pass_hash  TEXT NOT NULL DEFAULT '', -- leer = Profil ohne Passwort
	created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE sessions (
	hash      TEXT PRIMARY KEY, -- SHA-256 des Tokens, nie das Token selbst
	user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	device    TEXT NOT NULL DEFAULT '',
	last_seen INTEGER NOT NULL
) STRICT;
CREATE INDEX sessions_user ON sessions(user_id);

-- Bewusst ohne Fremdschlüssel auf items: Ist ein NAS-Laufwerk kurz weg, darf kein Fortschritt verloren gehen.
CREATE TABLE progress (
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	item_id    TEXT NOT NULL,
	pos        REAL NOT NULL DEFAULT 0,
	duration   REAL NOT NULL DEFAULT 0,
	watched    INTEGER NOT NULL DEFAULT 0,
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX progress_recent ON progress(user_id, updated_at DESC);

CREATE TABLE series_prefs (
	user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	series   TEXT NOT NULL,
	audio    TEXT NOT NULL DEFAULT '',
	subtitle TEXT NOT NULL DEFAULT '', -- 'off' = bewusst aus
	PRIMARY KEY (user_id, series)
) STRICT, WITHOUT ROWID;

CREATE TABLE devices (
	id         TEXT PRIMARY KEY,
	profile    TEXT NOT NULL, -- JSON, Format gehört playback
	updated_at INTEGER NOT NULL
) STRICT;

-- Katalog (gehört internal/scan)
CREATE TABLE items (
	id        TEXT PRIMARY KEY,
	path      TEXT NOT NULL UNIQUE,
	size      INTEGER NOT NULL,
	mtime     INTEGER NOT NULL, -- Nanosekunden, zum Erkennen geänderter Dateien
	added_at  INTEGER NOT NULL,
	kind      TEXT NOT NULL,    -- movie, episode
	title     TEXT NOT NULL,
	year      INTEGER NOT NULL DEFAULT 0,
	series    TEXT NOT NULL DEFAULT '',
	season    INTEGER NOT NULL DEFAULT 0,
	episode   INTEGER NOT NULL DEFAULT 0,
	container TEXT NOT NULL,
	duration  REAL NOT NULL,
	bitrate   INTEGER NOT NULL
) STRICT;
CREATE INDEX items_series ON items(series, season, episode);
CREATE INDEX items_added ON items(added_at DESC);

CREATE TABLE streams (
	item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	idx        INTEGER NOT NULL,
	type       TEXT NOT NULL,
	codec      TEXT NOT NULL,
	profile    TEXT NOT NULL DEFAULT '',
	pix_fmt    TEXT NOT NULL DEFAULT '',
	width      INTEGER NOT NULL DEFAULT 0,
	height     INTEGER NOT NULL DEFAULT 0,
	channels   INTEGER NOT NULL DEFAULT 0,
	language   TEXT NOT NULL DEFAULT '',
	title      TEXT NOT NULL DEFAULT '',
	is_default INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (item_id, idx)
) STRICT, WITHOUT ROWID;

-- Keyframe-Zeitstempel als float64 little-endian hintereinander; gilt nur für size/mtime der Datei.
CREATE TABLE keyframes (
	item_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
	size    INTEGER NOT NULL,
	mtime   INTEGER NOT NULL,
	data    BLOB NOT NULL
) STRICT;

-- Metadaten (gehört internal/meta). Ohne Fremdschlüssel: manuelle Korrekturen überleben ein kurz fehlendes Laufwerk.
CREATE TABLE meta (
	item_id    TEXT PRIMARY KEY,
	source     TEXT NOT NULL, -- manual, nfo, tmdb, filename
	json       TEXT NOT NULL,
	uncertain  INTEGER NOT NULL DEFAULT 0,
	updated_at INTEGER NOT NULL
) STRICT;
