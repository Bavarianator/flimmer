-- Untertitel-Kennzeichen für die automatische Wahl (nur fremdsprachige Stellen, Hörgeschädigte).
ALTER TABLE streams ADD COLUMN forced INTEGER NOT NULL DEFAULT 0;
ALTER TABLE streams ADD COLUMN hearing_impaired INTEGER NOT NULL DEFAULT 0;
