-- Favoriten pro Profil. key ist eine Titel-ID oder "serie:<Name>"; ohne Fremdschlüssel auf items (wie progress).
CREATE TABLE IF NOT EXISTS favorites (
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	key        TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, key)
) STRICT, WITHOUT ROWID;
