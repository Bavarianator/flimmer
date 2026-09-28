package party

import (
	"math"
	"testing"
	"time"
)

type step struct {
	at     float64 // Sekunden nach t0
	member string
	a      Action
	expire bool // statt Aktion: Expire aufrufen
	leave  bool
}

type want struct {
	at      float64
	pos     float64
	paused  bool
	waiting int
}

func TestRoom(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	at := func(s float64) time.Time { return t0.Add(time.Duration(s * float64(time.Second))) }
	tests := []struct {
		name  string
		steps []step
		want  want
	}{
		{"neu: pausiert bei 0", nil, want{at: 30, pos: 0, paused: true}},
		{"play läuft mit", []step{{at: 0, a: Action{Type: "play"}}}, want{at: 10, pos: 10}},
		{"pause hält fest", []step{{at: 0, a: Action{Type: "play"}}, {at: 10, a: Action{Type: "pause"}}}, want{at: 60, pos: 10, paused: true}},
		{"seek beim Abspielen", []step{{at: 0, a: Action{Type: "play"}}, {at: 5, a: Action{Type: "seek", Pos: 100}}}, want{at: 10, pos: 105}},
		{"Geschwindigkeit", []step{{at: 0, a: Action{Type: "play"}}, {at: 10, a: Action{Type: "rate", Rate: 1.5}}}, want{at: 20, pos: 25}},
		{"Puffern hält den Raum an", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
		}, want{at: 8, pos: 5, paused: true, waiting: 1}},
		{"erst wenn alle fertig sind, geht es weiter", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
			{at: 6, member: "b", a: Action{Type: "buffering", Buffering: true}},
			{at: 7, member: "a", a: Action{Type: "buffering"}},
			{at: 8, member: "b", a: Action{Type: "buffering"}},
		}, want{at: 10, pos: 7}},
		{"nach 10 s Puffern geht es ohne ihn weiter", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
			{at: 15, expire: true},
		}, want{at: 17, pos: 7}},
		{"wer geht, blockiert nicht mehr", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
			{at: 6, member: "a", leave: true},
		}, want{at: 9, pos: 8}},
		{"Pause bleibt nach dem Puffern", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
			{at: 6, a: Action{Type: "pause"}},
			{at: 7, member: "a", a: Action{Type: "buffering"}},
		}, want{at: 20, pos: 5, paused: true}},
		{"neuer Titel beginnt pausiert bei 0", []step{
			{at: 0, a: Action{Type: "play"}},
			{at: 5, member: "a", a: Action{Type: "buffering", Buffering: true}},
			{at: 9, a: Action{Type: "media", MediaID: "neu"}},
		}, want{at: 20, pos: 0, paused: true}},
		{"Chat ändert nichts", []step{{at: 0, a: Action{Type: "play"}}, {at: 3, a: Action{Type: "chat", Text: "hi"}}}, want{at: 4, pos: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRoom("film", "host", t0)
			for _, s := range tt.steps {
				switch {
				case s.expire:
					r.Expire(at(s.at))
				case s.leave:
					r.Leave(s.member, at(s.at))
				default:
					if _, err := r.Apply(s.member, s.a, at(s.at)); err != nil {
						t.Fatal(err)
					}
				}
			}
			st := r.State(at(tt.want.at))
			if math.Abs(st.Pos-tt.want.pos) > 1e-9 || st.Paused != tt.want.paused || len(st.Waiting) != tt.want.waiting {
				t.Fatalf("got pos=%v paused=%v waiting=%v, want %+v", st.Pos, st.Paused, st.Waiting, tt.want)
			}
		})
	}
}

func TestRoomRejects(t *testing.T) {
	r := NewRoom("film", "host", time.Now())
	for _, a := range []Action{{Type: "seek", Pos: -1}, {Type: "rate", Rate: 10}, {Type: "media"}, {Type: "unsinn"}} {
		if _, err := r.Apply("a", a, time.Now()); err == nil {
			t.Errorf("%+v akzeptiert", a)
		}
	}
}
