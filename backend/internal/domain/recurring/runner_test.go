package recurring

import (
	"testing"
	"time"
)

func TestDueDate(t *testing.T) {
	tests := []struct {
		name       string
		today      string
		dayOfMonth int
		want       string
	}{
		{name: "after day uses current month", today: "2026-05-15", dayOfMonth: 10, want: "2026-05-10"},
		{name: "before day uses previous month", today: "2026-05-05", dayOfMonth: 10, want: "2026-04-10"},
		{name: "on day uses current month", today: "2026-05-10", dayOfMonth: 10, want: "2026-05-10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			today := mustDate(t, tt.today)
			got := DueDate(today, tt.dayOfMonth)
			if got.Format("2006-01-02") != tt.want {
				t.Fatalf("DueDate() = %s, want %s", got.Format("2006-01-02"), tt.want)
			}
		})
	}
}

func TestShouldGenerate(t *testing.T) {
	due := mustDate(t, "2026-05-10")
	tests := []struct {
		name          string
		lastGenerated *time.Time
		want          bool
	}{
		{name: "nil last generated", want: true},
		{name: "before due", lastGenerated: datePtrForTest(t, "2026-05-09"), want: true},
		{name: "same due", lastGenerated: datePtrForTest(t, "2026-05-10"), want: false},
		{name: "after due", lastGenerated: datePtrForTest(t, "2026-05-11"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldGenerate(due, tt.lastGenerated); got != tt.want {
				t.Fatalf("ShouldGenerate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustDate(t *testing.T, raw string) time.Time {
	t.Helper()
	date, err := time.Parse("2006-01-02", raw)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func datePtrForTest(t *testing.T, raw string) *time.Time {
	t.Helper()
	date := mustDate(t, raw)
	return &date
}
