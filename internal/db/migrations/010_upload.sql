-- Recht „Hochladen“ pro Benutzer; vergibt der Admin (Dashboard › Benutzer). Admins dürfen immer.
ALTER TABLE users ADD COLUMN upload INTEGER NOT NULL DEFAULT 0;
