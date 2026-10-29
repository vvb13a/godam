package api

import (
	"fmt"
	"time"
)

func (s *Server) cacheControl(visibility string) string {
	ttl := s.deps.DeliveryCacheTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return fmt.Sprintf("%s, max-age=%d", visibility, int64(ttl/time.Second))
}
