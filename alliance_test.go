package social

import (
	"testing"
	"time"
)

func TestBetrayal(t *testing.T) {
	a := &Alliance{ID: "a1"}
	_ = a.Join("p1")
	_ = a.Join("p2")
	now := time.Now()
	if Hostile(a, "p1", "p2", now) || !SharesVision(a, "p1", "p2", now) {
		t.Fatal("aliados ativos")
	}
	if err := a.Betray("p1", now); err != nil {
		t.Fatal(err)
	}
	if SharesVision(a, "p1", "p2", now) {
		t.Fatal("visão deve cair no mesmo tick")
	}
	if Hostile(a, "p1", "p2", now.Add(time.Second)) {
		t.Fatal("fogo amigo só após a carência")
	}
	if !Hostile(a, "p1", "p2", now.Add(FriendlyFireGrace)) {
		t.Fatal("fogo amigo deve ligar após a carência")
	}
	if !IsTraitorMarked(a, "p1", now.Add(10*time.Minute)) || IsTraitorMarked(a, "p2", now) {
		t.Fatal("marca de traidor")
	}
}
