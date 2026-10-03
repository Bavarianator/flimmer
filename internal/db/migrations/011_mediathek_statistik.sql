-- Sehzeit pro Profil, Titel und Stunde; die progress-Herzschläge zählen hoch (Dashboard › Statistik).
CREATE TABLE IF NOT EXISTS watch (
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	item_id TEXT NOT NULL,
	hour    INTEGER NOT NULL,         -- Unix-ms, auf die volle Stunde abgerundet
	seconds REAL NOT NULL DEFAULT 0,
	method  TEXT NOT NULL DEFAULT '', -- direct | remux | transcode
	client  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (user_id, item_id, hour)
) STRICT, WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS watch_hour ON watch(hour);

-- Alle Downloads (mediathek | abo | link | upload); Mediathek-Einträge sind zugleich die Warteschlange.
CREATE TABLE IF NOT EXISTS downloads (
	id         INTEGER PRIMARY KEY,
	source     TEXT NOT NULL,
	ext_id     TEXT UNIQUE,              -- MediathekViewWeb-ID, verhindert Doppel-Downloads
	user_id    TEXT NOT NULL DEFAULT '',
	title      TEXT NOT NULL,
	channel    TEXT NOT NULL DEFAULT '',
	url        TEXT NOT NULL DEFAULT '',
	file       TEXT NOT NULL DEFAULT '',
	bytes      INTEGER NOT NULL DEFAULT 0, -- erwartete Größe, nach dem Laden die echte
	status     TEXT NOT NULL,            -- wartet | laeuft | fertig | fehler | abgebrochen
	error      TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	done_at    INTEGER
) STRICT;
CREATE INDEX IF NOT EXISTS downloads_status ON downloads(status, id);

CREATE TABLE IF NOT EXISTS mediathek_abos (
	id          INTEGER PRIMARY KEY,
	query       TEXT NOT NULL,
	channel     TEXT NOT NULL DEFAULT '',
	min_minutes INTEGER NOT NULL DEFAULT 0,
	created_at  INTEGER NOT NULL
) STRICT;
