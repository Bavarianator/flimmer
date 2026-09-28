// Package lang vereinheitlicht Sprachkennungen. MKV-Tags nutzen ISO 639-2/B („ger“), MP4 639-2/T („deu“),
// Dateinamen oft 639-1 („de“) oder Namen („German“, „Deutsch“). Verglichen wird immer in 639-2/B.
package lang

import "strings"

// Tabelle: 639-2/B → weitere Schreibweisen. ponytail: die ~25 häufigsten Sprachen; ergänzen, wenn eine fehlt
var table = map[string][]string{
	"ger": {"de", "deu", "german", "deutsch"},
	"eng": {"en", "english", "englisch"},
	"fre": {"fr", "fra", "french", "français", "francais", "französisch"},
	"spa": {"es", "spanish", "español", "espanol", "spanisch"},
	"ita": {"it", "italian", "italiano", "italienisch"},
	"dut": {"nl", "nld", "dutch", "nederlands", "niederländisch"},
	"por": {"pt", "portuguese", "português", "portugiesisch"},
	"rus": {"ru", "russian", "русский", "russisch"},
	"pol": {"pl", "polish", "polski", "polnisch"},
	"tur": {"tr", "turkish", "türkçe", "türkisch"},
	"jpn": {"ja", "japanese", "日本語", "japanisch"},
	"kor": {"ko", "korean", "한국어", "koreanisch"},
	"chi": {"zh", "zho", "chinese", "中文", "chinesisch"},
	"swe": {"sv", "swedish", "svenska", "schwedisch"},
	"dan": {"da", "danish", "dansk", "dänisch"},
	"nor": {"no", "nb", "nob", "norwegian", "norsk", "norwegisch"},
	"fin": {"fi", "finnish", "suomi", "finnisch"},
	"cze": {"cs", "ces", "czech", "čeština", "tschechisch"},
	"hun": {"hu", "hungarian", "magyar", "ungarisch"},
	"gre": {"el", "ell", "greek", "ελληνικά", "griechisch"},
	"ara": {"ar", "arabic", "العربية", "arabisch"},
	"heb": {"he", "hebrew", "עברית", "hebräisch"},
	"hin": {"hi", "hindi"},
	"ukr": {"uk", "ukrainian", "українська", "ukrainisch"},
	"ron": {"ro", "rum", "romanian", "română", "rumänisch"},
}

var index = func() map[string]string {
	m := map[string]string{}
	for b, alts := range table {
		m[b] = b
		for _, a := range alts {
			m[a] = b
		}
	}
	return m
}()

// Normalize liefert den 639-2/B-Code oder "" für unbekannt/undefiniert („und“, leer).
// Regionen werden abgeschnitten: „pt-BR“ → „por“.
func Normalize(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(t, "-_"); i > 0 {
		t = t[:i]
	}
	return index[t]
}

// Pick wählt aus den Sprachen der Spuren die beste nach der Fallback-Kette prefs (z. B. ["ger", "eng"]).
// Rückgabe ist der Index in tracks oder -1, wenn keine Sprache der Kette vorkommt.
func Pick(tracks []string, prefs []string) int {
	for _, p := range prefs {
		p = Normalize(p)
		if p == "" {
			continue
		}
		for i, t := range tracks {
			if Normalize(t) == p {
				return i
			}
		}
	}
	return -1
}
