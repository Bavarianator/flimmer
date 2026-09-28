package lang

import "testing"

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"ger": "ger", "deu": "ger", "de": "ger", "German": "ger", "Deutsch": "ger", " DE ": "ger",
		"en": "eng", "eng": "eng", "pt-BR": "por", "zh_Hans": "chi", "und": "", "": "", "klingon": "",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, erwartet %q", in, got, want)
		}
	}
}

func TestPick(t *testing.T) {
	tracks := []string{"eng", "deu", "jpn"}
	for _, c := range []struct {
		prefs []string
		want  int
	}{
		{[]string{"de", "en"}, 1},
		{[]string{"fr", "en"}, 0},
		{[]string{"ja"}, 2},
		{[]string{"fr"}, -1},
		{nil, -1},
	} {
		if got := Pick(tracks, c.prefs); got != c.want {
			t.Errorf("Pick(%v) = %d, erwartet %d", c.prefs, got, c.want)
		}
	}
}
