package audio

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFilter(t *testing.T) {
	for _, c := range []struct {
		in, out int
		night   bool
		want    string
	}{
		{6, 2, false, downmix},
		{8, 2, true, downmix + "," + night},
		{2, 2, false, ""},
		{6, 6, false, ""},
		{2, 2, true, night},
	} {
		if got := Filter(c.in, c.out, c.night); got != c.want {
			t.Errorf("Filter(%d, %d, %v) = %q", c.in, c.out, c.night, got)
		}
	}
}

// TestFFmpeg prüft, dass ffmpeg die Ketten für 5.1, 5.1(side) und 7.1 annimmt und Stereo herauskommt.
func TestFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	for layout, n := range map[string]int{"5.1": 6, "5.1(side)": 6, "7.1": 8} {
		ch := make([]string, n)
		for i := range ch {
			ch[i] = "0.3*sin(440*2*PI*t)"
		}
		src := "aevalsrc=" + strings.Join(ch, "|") + ":c=" + layout + ":d=1"
		out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", src,
			"-af", Filter(n, 2, true), "-ac", "2", "-c:a", "aac", "-f", "null", "-").CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v %s", layout, err, out)
		}
	}
}
