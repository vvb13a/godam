package api

import (
	"net/http"

	"github.com/vvb13a/godam/domain"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.deps.Stats.GetScoped(
		r.Context(),
		enforcedAttributes(domain.ActorFromContext(r.Context())),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// enforcedAttributes returns the locked and hidden attribute filters that must
// be applied to background aggregates such as stats.
func enforcedAttributes(actor *domain.Actor) map[string][]string {
	if actor == nil || !actor.Token {
		return nil
	}
	attributes := make(map[string][]string)
	for _, grant := range actor.Scope.Attributes {
		if grant.Mode == domain.AttributeModeLocked || grant.Mode == domain.AttributeModeHidden {
			attributes[grant.Key] = append([]string{}, grant.Values...)
		}
	}
	return attributes
}
