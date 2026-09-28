-- Probe-Version: Ändert sich, was probe liefert, wird beim nächsten Scan neu geprobt (nur der schnelle Header-Probe).
ALTER TABLE items ADD COLUMN probe_version INTEGER NOT NULL DEFAULT 0;

-- HDR/Dolby Vision für die 4K/HDR-Entscheidung.
ALTER TABLE streams ADD COLUMN hdr TEXT NOT NULL DEFAULT '';        -- '', hdr10, hdr10+, hlg, dv
ALTER TABLE streams ADD COLUMN dv_profile INTEGER NOT NULL DEFAULT 0;
ALTER TABLE streams ADD COLUMN dv_compat INTEGER NOT NULL DEFAULT 0;
