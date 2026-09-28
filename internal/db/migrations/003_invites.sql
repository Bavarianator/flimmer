-- Einladungen (gehört internal/share). Gespeichert wird nur der SHA-256 des Token-Kerns, nie das Token selbst.
CREATE TABLE IF NOT EXISTS invites (
	id         TEXT PRIMARY KEY,           -- hex(SHA-256(Token-Kern))
	created_by TEXT NOT NULL,              -- Benutzer-ID des Admins; ohne Fremdschlüssel, Einladungen überleben ihn
	note       TEXT NOT NULL DEFAULT '',
	scope      TEXT NOT NULL,              -- JSON {"libraries":[Pfad…],"items":[ID…]}
	expires    INTEGER NOT NULL,           -- Unix-ms; danach weder einlösbar noch nutzbar
	max_uses   INTEGER NOT NULL DEFAULT 0, -- 0 = unbegrenzt
	uses       INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
) STRICT;

-- Gast-Benutzer einer Einladung. Widerruf löscht erst die Benutzer (samt Sessions), dann die Einladung.
CREATE TABLE IF NOT EXISTS guests (
	user_id   TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	invite_id TEXT NOT NULL REFERENCES invites(id) ON DELETE CASCADE
) STRICT;
CREATE INDEX IF NOT EXISTS guests_invite ON guests(invite_id);
