-- Vorgemerkte Optimierung: Titel, die man in den Menüs „Umwandeln“ angemeldet hat.
-- Der Optimizer nimmt sie zusätzlich zu den Ampelroten in sein Programm auf.
CREATE TABLE IF NOT EXISTS opt_queue (
	id   TEXT PRIMARY KEY,
	since INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
