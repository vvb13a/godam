package api

import (
	"testing"
	"time"
)

func TestParseDatePtr(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		endOfDay bool
		want     time.Time
	}{
		{
			name: "date only",
			raw:  "2026-01-02",
			want: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "date only end of day",
			raw:      "2026-01-02",
			endOfDay: true,
			want:     time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "datetime without seconds",
			raw:  "2026-01-02T15:04",
			want: time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC),
		},
		{
			name:     "datetime end of day does not add a day",
			raw:      "2026-01-02T15:04",
			endOfDay: true,
			want:     time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC),
		},
		{
			name: "datetime with space",
			raw:  "2026-01-02 15:04",
			want: time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC),
		},
		{
			name: "rfc3339",
			raw:  "2026-01-02T15:04:05Z",
			want: time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseDatePtr(test.raw, test.endOfDay)
			if got == nil {
				t.Fatal("expected a parsed time")
			}
			if !got.Equal(test.want) {
				t.Fatalf("parseDatePtr(%q) = %v, want %v", test.raw, got, test.want)
			}
		})
	}

	if parseDatePtr("", false) != nil {
		t.Fatal("empty value should be nil")
	}
	if parseDatePtr("not-a-date", false) != nil {
		t.Fatal("invalid value should be nil")
	}
}
