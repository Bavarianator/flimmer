package party

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Zwei Geräte im Raum: Play kommt beim anderen an, Puffern hält alle an, Chat wird verteilt.
func TestHub(t *testing.T) {
	h := &Hub{User: func(r *http.Request) (string, string, bool) {
		u := r.Header.Get("X-User")
		return u, strings.ToUpper(u[:1]) + u[1:], u != ""
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /party", h.Create)
	mux.HandleFunc("GET /party/{id}/events", h.Events)
	mux.HandleFunc("POST /party/{id}/actions", h.Action)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // läuft vor srv.Close: offene SSE-Verbindungen beenden, sonst wartet Close ewig

	do := func(user, method, path, body string) *http.Response {
		req, _ := http.NewRequestWithContext(ctx, method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("X-User", user)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	var created struct{ ID string }
	json.NewDecoder(do("anna", "POST", "/party", `{"mediaId":"film"}`).Body).Decode(&created)

	type ev struct {
		name string
		data map[string]any
	}
	connect := func(user string) (string, chan ev) {
		res := do(user, "GET", "/party/"+created.ID+"/events", "")
		ch := make(chan ev, 32)
		go func() {
			sc := bufio.NewScanner(res.Body)
			var name string
			for sc.Scan() {
				line := sc.Text()
				if n, ok := strings.CutPrefix(line, "event: "); ok {
					name = n
				} else if d, ok := strings.CutPrefix(line, "data: "); ok {
					var m map[string]any
					json.Unmarshal([]byte(d), &m)
					ch <- ev{name, m}
				}
			}
		}()
		hello := <-ch
		if hello.name != "hello" {
			t.Fatalf("erstes Ereignis %q", hello.name)
		}
		return hello.data["member"].(string), ch
	}
	next := func(ch chan ev, name string) map[string]any {
		t.Helper()
		for {
			select {
			case e := <-ch:
				if e.name == name {
					return e.data
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("kein %q-Ereignis", name)
			}
		}
	}
	// until liest state-Ereignisse, bis cond passt (jeder bekommt auch die Folgen eigener Aktionen).
	until := func(ch chan ev, cond func(st map[string]any) bool) map[string]any {
		t.Helper()
		for {
			if st := next(ch, "state")["state"].(map[string]any); cond(st) {
				return st
			}
		}
	}

	annaM, anna := connect("anna")
	benM, ben := connect("ben")
	for len(next(anna, "members")["members"].([]any)) != 2 { // erst Annas eigener Beitritt, dann Ben
	}

	do("anna", "POST", "/party/"+created.ID+"/actions", `{"member":"`+annaM+`","type":"play"}`)
	until(ben, func(st map[string]any) bool { return st["paused"] == false }) // Play kommt beim anderen an
	do("ben", "POST", "/party/"+created.ID+"/actions", `{"member":"`+benM+`","type":"buffering","buffering":true}`)
	if st := until(anna, func(st map[string]any) bool { return len(st["waiting"].([]any)) == 1 }); st["paused"] != true {
		t.Fatalf("Puffern hält nicht an: %v", st)
	}
	do("ben", "POST", "/party/"+created.ID+"/actions", `{"member":"`+benM+`","type":"chat","text":"Popcorn!"}`)
	if c := next(anna, "chat"); c["text"] != "Popcorn!" || c["from"] != "Ben" {
		t.Fatalf("Chat: %v", c)
	}
	if res := do("ben", "POST", "/party/"+created.ID+"/actions", `{"member":"`+annaM+`","type":"pause"}`); res.StatusCode != http.StatusForbidden {
		t.Fatalf("fremdes Mitglied: %d", res.StatusCode)
	}
}
