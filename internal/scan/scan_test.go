package scan

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		path string
		want Item
	}{
		{"/m/Filme/Inception.2010.1080p.BluRay.x264.mkv", Item{Title: "Inception", Year: 2010}},
		{"/m/Filme/Der Pate (1972).mp4", Item{Title: "Der Pate", Year: 1972}},
		{"/m/Filme/Blade Runner 2049 (2017)/Blade Runner 2049 (2017).mkv", Item{Title: "Blade Runner 2049", Year: 2017}},
		{"/m/Filme/Heat.mkv", Item{Title: "Heat"}},
		{"/m/Serien/Dark.S01E03.Vergangenheit.und.Gegenwart.German.1080p.mkv", Item{Series: "Dark", Season: 1, Episode: 3, Title: "Vergangenheit und Gegenwart"}},
		{"/m/Serien/Breaking Bad/Staffel 2/S02E05.mkv", Item{Series: "Breaking Bad", Season: 2, Episode: 5, Title: "Episode 5"}},
		{"/m/Serien/The Office/the_office_s3e12_720p.mkv", Item{Series: "the office", Season: 3, Episode: 12, Title: "Episode 12"}},
	}
	for _, tt := range tests {
		if got := Parse(tt.path); got != tt.want {
			t.Errorf("Parse(%q)\n got  %+v\n want %+v", tt.path, got, tt.want)
		}
	}
}
