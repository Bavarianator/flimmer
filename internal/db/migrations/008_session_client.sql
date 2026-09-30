-- Geräteliste im Dashboard: womit und von wo eine Session angelegt wurde.
ALTER TABLE sessions ADD COLUMN client TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN ip TEXT NOT NULL DEFAULT '';
