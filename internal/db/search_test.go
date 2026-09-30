package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSearch(t *testing.T) {
	ctx := context.Background()
	d, err := Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	add := func(id, title, series, metaJSON string) {
		t.Helper()
		if _, err := d.Exec(`INSERT INTO items(id, path, size, mtime, added_at, kind, title, series, container, duration, bitrate)
			VALUES(?, ?, 1, 1, 1, 'movie', ?, ?, 'mkv', 1, 1)`, id, "/m/"+id, title, series); err != nil {
			t.Fatal(err)
		}
		if metaJSON != "" {
			if _, err := d.Exec(`INSERT INTO meta(item_id, source, json, updated_at) VALUES(?, 'tmdb', ?, 1)`, id, metaJSON); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("lotr", "Herr der Ringe", "", `{"title":"Der Herr der Ringe: Die Gefährten","originalTitle":"The Lord of the Rings"}`)
	add("hp", "Harry Potter", "", "")
	add("ep", "Pilot", "Breaking Bad", "")
	add("up", "Up", "", "kaputt{") // kaputtes JSON bricht nichts
	add("amelie", "Die fabelhafte Welt der Amélie", "", "")

	ids := func(q string) []string {
		t.Helper()
		hits, err := d.Search(ctx, q)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		var out []string
		for _, h := range hits {
			out = append(out, h.ID)
		}
		return out
	}
	for _, c := range []struct{ q, want string }{
		{"her der ringe", "lotr"},
		{"lord of the rings", "lotr"}, // Originaltitel aus den Metadaten
		{"gefährten", "lotr"},
		{"hary poter", "hp"}, // Tippfehler
		{"breaking", "ep"},   // Serie
		{"up", "up"},         // kurz: Teilstring
		{"amelie", "amelie"}, // ohne Akzent
	} {
		if got := ids(c.q); len(got) == 0 || got[0] != c.want {
			t.Errorf("%q: %v, erwartet zuerst %s", c.q, got, c.want)
		}
	}
	if got := ids("xyzqw"); len(got) != 0 {
		t.Errorf("Unsinn findet %v", got)
	}

	// Trigger: Metadaten ändern und Titel löschen halten den Index aktuell.
	d.Exec(`UPDATE meta SET json = '{"originalTitle":"Il Signore degli Anelli"}' WHERE item_id = 'lotr'`)
	if got := ids("signore anelli"); len(got) == 0 || got[0] != "lotr" {
		t.Errorf("nach Meta-Änderung: %v", got)
	}
	if got := ids("lord rings"); len(got) != 0 {
		t.Errorf("alter Originaltitel noch im Index: %v", got)
	}
	d.Exec(`DELETE FROM items WHERE id = 'hp'`)
	if got := ids("harry potter"); len(got) != 0 {
		t.Errorf("gelöschter Titel gefunden: %v", got)
	}
}
