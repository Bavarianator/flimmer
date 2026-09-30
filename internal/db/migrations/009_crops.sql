-- Eingebrannte schwarze Balken (cropdetect), Anteile 0..1. w = 0 heißt „keine Balken“, damit nichts doppelt läuft.
-- Gilt für die Datei, wie sie geprobt wurde: Ein neuer Probe (saveItem) löscht die Zeile.
CREATE TABLE IF NOT EXISTS crops (
	item_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
	x       REAL NOT NULL,
	y       REAL NOT NULL,
	w       REAL NOT NULL,
	h       REAL NOT NULL
) STRICT;
