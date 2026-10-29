package main

import (
	"testing"
	"time"
)

func TestDeliveryCacheTTL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{name: "default", raw: "", want: 24 * time.Hour},
		{name: "clamp minimum", raw: "1", want: time.Minute},
		{name: "clamp maximum", raw: "9999999", want: 7 * 24 * time.Hour},
		{name: "invalid", raw: "soon", want: 24 * time.Hour},
		{name: "explicit", raw: "3600", want: time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GODAM_DELIVERY_CACHE_TTL", test.raw)
			if got := deliveryCacheTTL(); got != test.want {
				t.Fatalf("deliveryCacheTTL() = %s, want %s", got, test.want)
			}
		})
	}
}
