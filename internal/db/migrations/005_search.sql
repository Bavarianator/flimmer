-- Volltextsuche über Titel, Originaltitel und Serie (Datei und Metadaten). Trigramme finden Wortteile,
-- deshalb findet „her der ringe“ auch „Herr der Ringe“. rowid = items.rowid; Trigger halten den Index aktuell.
CREATE VIRTUAL TABLE IF NOT EXISTS search USING fts5(title, original, series, tokenize = 'trigram remove_diacritics 1');

-- Kaputtes Meta-JSON darf den Scan nie aufhalten, daher json_valid.
CREATE VIEW IF NOT EXISTS search_src AS
SELECT i.rowid AS rid, i.id AS id,
	i.title || ' ' || coalesce(CASE WHEN json_valid(m.json) THEN m.json ->> '$.title' END, '') AS title,
	coalesce(CASE WHEN json_valid(m.json) THEN m.json ->> '$.originalTitle' END, '') AS original,
	i.series || ' ' || coalesce(CASE WHEN json_valid(m.json) THEN m.json ->> '$.series' END, '') AS series
FROM items i LEFT JOIN meta m ON m.item_id = i.id;

CREATE TRIGGER IF NOT EXISTS search_items_ins AFTER INSERT ON items BEGIN
	INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src WHERE id = new.id;
END;
CREATE TRIGGER IF NOT EXISTS search_items_upd AFTER UPDATE OF title, series ON items BEGIN
	DELETE FROM search WHERE rowid = old.rowid;
	INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src WHERE id = new.id;
END;
CREATE TRIGGER IF NOT EXISTS search_items_del AFTER DELETE ON items BEGIN
	DELETE FROM search WHERE rowid = old.rowid;
END;
CREATE TRIGGER IF NOT EXISTS search_meta_ins AFTER INSERT ON meta BEGIN
	DELETE FROM search WHERE rowid = (SELECT rowid FROM items WHERE id = new.item_id);
	INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src WHERE id = new.item_id;
END;
CREATE TRIGGER IF NOT EXISTS search_meta_upd AFTER UPDATE ON meta BEGIN
	DELETE FROM search WHERE rowid = (SELECT rowid FROM items WHERE id = new.item_id);
	INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src WHERE id = new.item_id;
END;
CREATE TRIGGER IF NOT EXISTS search_meta_del AFTER DELETE ON meta BEGIN
	DELETE FROM search WHERE rowid = (SELECT rowid FROM items WHERE id = old.item_id);
	INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src WHERE id = old.item_id;
END;

-- Idempotent wie 003: Der Index wird bei jedem Lauf neu gefüllt.
DELETE FROM search;
INSERT INTO search(rowid, title, original, series) SELECT rid, title, original, series FROM search_src;
