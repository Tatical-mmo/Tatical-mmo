package progression

import (
	"testing"
	"time"
)

func TestTripleCap(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	week4 := start.Add(24 * time.Hour * 7 * 3)
	st := ServerTier(start, week4, 0)
	if st != 3 {
		t.Fatalf("semana 4 deveria liberar T3, veio T%d", st)
	}
	p := PlayerState{ResearchedTier: 5, HQLevel: 12}
	if got := MaxAllowedLevel(HQ, p, st); got != 15 {
		t.Fatalf("QG limitado pelo tier do servidor: %d", got)
	}
	if got := MaxAllowedLevel(Building, p, st); got != 12 {
		t.Fatalf("construção limitada pelo QG: %d", got)
	}
	if err := CanUpgrade(Troop, 12, p, st); err != ErrAboveCap {
		t.Fatal("tropa não pode passar do QG")
	}
	if ServerTier(start, start, 5) != 5 {
		t.Fatal("override do Admin")
	}
}

func TestCatchUpDiscount(t *testing.T) {
	full := Cost(100, 3, PlayerState{ResearchedTier: 3}, 3)
	disc := Cost(100, 3, PlayerState{ResearchedTier: 1}, 3)
	if full != 256 || disc != 192 {
		t.Fatalf("full=%d disc=%d", full, disc)
	}
}
