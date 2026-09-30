package db

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

// FTS5 steckt nicht im Kern-SQLite des Treibers; als Auto-Erweiterung gilt es für jede Verbindung,
// auch für fremde Schreiber auf items/meta (Scan, meta), deren Trigger den Suchindex pflegen.
func init() { sqlite3.AutoExtension(fts5.Register) }

// SearchHit ist ein Treffer der Suche; Score ist der Anteil der Such-Trigramme, die im Titel vorkommen (0–1).
type SearchHit struct {
	ID    string
	Score float64
}

const (
	searchCandidates = 1000 // Vorauswahl aus FTS5, danach bewertet Go
	searchMinScore   = 0.5  // mindestens die Hälfte der Trigramme muss passen (toleriert Tippfehler)
)

// Search sucht fehlertolerant über Titel, Originaltitel und Serie, beste Treffer zuerst.
// FTS5 liefert Kandidaten mit irgendeinem gemeinsamen Trigramm, Go bewertet den Anteil passender Trigramme.
// Anfragen ohne Wort ab drei Zeichen („Up“, „ES“) suchen per LIKE als Teilstring.
func (d *DB) Search(ctx context.Context, q string) ([]SearchHit, error) {
	grams := trigrams(q)
	var query string
	var args []any
	if len(grams) == 0 {
		w := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.TrimSpace(q)) + "%"
		if w == "%%" {
			return nil, nil
		}
		query = `SELECT i.id, s.title, s.original, s.series FROM search s JOIN items i ON i.rowid = s.rowid
			WHERE s.title LIKE ?1 ESCAPE '\' OR s.original LIKE ?1 ESCAPE '\' OR s.series LIKE ?1 ESCAPE '\' LIMIT ?2`
		args = []any{w, searchCandidates}
	} else {
		quoted := make([]string, len(grams))
		for i, g := range grams {
			quoted[i] = `"` + strings.ReplaceAll(g, `"`, `""`) + `"`
		}
		query = `SELECT i.id, s.title, s.original, s.series FROM search s JOIN items i ON i.rowid = s.rowid
			WHERE search MATCH ? ORDER BY rank LIMIT ?`
		args = []any{strings.Join(quoted, " OR "), searchCandidates}
	}
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var id, title, orig, series string
		if err := rows.Scan(&id, &title, &orig, &series); err != nil {
			return nil, err
		}
		score := 1.0
		if len(grams) > 0 {
			score = match(grams, fold(title+" "+orig+" "+series))
		}
		if score >= searchMinScore {
			out = append(out, SearchHit{id, score})
		}
	}
	slices.SortStableFunc(out, func(a, b SearchHit) int { return cmp.Compare(b.Score, a.Score) })
	return out, rows.Err()
}

// trigrams liefert die Trigramme jedes Wortes (ohne Wortgrenzen-Trigramme, doppelte nur einmal).
func trigrams(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		rs := []rune(w)
		for i := 0; i+3 <= len(rs); i++ {
			if g := string(rs[i : i+3]); !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	return out
}

func match(grams []string, text string) float64 {
	n := 0
	for _, g := range grams {
		if strings.Contains(text, g) {
			n++
		}
	}
	return float64(n) / float64(len(grams))
}

// fold entspricht grob remove_diacritics im Tokenizer: klein, ohne gängige Akzente.
var diacritics = strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "á", "a", "à", "a", "â", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "ú", "u", "ù", "u", "û", "u", "ñ", "n", "ç", "c", "å", "a", "ø", "o")

func fold(s string) string { return diacritics.Replace(strings.ToLower(s)) }
