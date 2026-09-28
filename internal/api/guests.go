package api

import (
	"context"
	"net/http"

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/share"
)

// Gäste (Einladungen, internal/share) sehen nur, was ihre Einladung erlaubt. Der Scope hängt am Request;
// nil = normaler Benutzer ohne Einschränkung.

type scopeKey struct{}

func scopeFrom(r *http.Request) *share.Scope {
	sc, _ := r.Context().Value(scopeKey{}).(*share.Scope)
	return sc
}

// withScope lädt den Scope eines Gastes; false = Einladung abgelaufen oder widerrufen.
func (s *Server) withScope(r *http.Request, u *db.User) (*http.Request, bool) {
	if s.Share == nil || u == nil {
		return r, true
	}
	sc, err := s.Share.Scope(r.Context(), u.ID)
	if err != nil {
		return r, false
	}
	if sc == nil {
		return r, true
	}
	return r.WithContext(context.WithValue(r.Context(), scopeKey{}, sc)), true
}

func allowed(r *http.Request, it *scan.Item) bool { return scopeFrom(r).Allows(it.ID, it.Path) }

// visible filtert eine Titelliste für Gäste (ohne Gast: dieselbe Liste, keine Kopie).
func visible(r *http.Request, list []*scan.Item) []*scan.Item {
	sc := scopeFrom(r)
	if sc == nil {
		return list
	}
	var out []*scan.Item
	for _, it := range list {
		if sc.Allows(it.ID, it.Path) {
			out = append(out, it)
		}
	}
	return out
}

// scopedLib ist die Bibliothek aus Sicht eines Gastes (für homeRows).
type scopedLib struct {
	lib *scan.Library
	r   *http.Request
}

func (l scopedLib) Get(id string) *scan.Item {
	if it := l.lib.Get(id); it != nil && allowed(l.r, it) {
		return it
	}
	return nil
}

func (l scopedLib) Episodes(series string) []*scan.Item { return visible(l.r, l.lib.Episodes(series)) }
func (l scopedLib) Newest() []*scan.Item                { return visible(l.r, l.lib.Newest()) }

func (s *Server) isGuest(ctx context.Context, id string) bool {
	return s.Share != nil && s.Share.IsGuest(ctx, id)
}

// GuestLogin meldet einen frisch eingeladenen Gast an (Callback für share.Options.Login).
func (s *Server) GuestLogin(w http.ResponseWriter, r *http.Request, u db.User) error {
	_, err := s.newSession(w, r, u.ID, "Einladung")
	return err
}
