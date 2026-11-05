package world

import "testing"

func TestDirectoryExpand(t *testing.T) {
	d := NewDirectory(1, func(SectorCoord) SectorType { return "plateau" })
	if len(d.Sectors) != 9 {
		t.Fatalf("raio 1 = 3x3, veio %d", len(d.Sectors))
	}
	added := d.ExpandRing(func(SectorCoord) SectorType { return "desert" })
	if len(added) != 16 || len(d.Sectors) != 25 {
		t.Fatalf("anel 2: %d novos, %d total", len(added), len(d.Sectors))
	}
}

func TestGhostsAndVisibility(t *testing.T) {
	if n := NeighborsForGhost(Vec2{10, 1000}); len(n) != 1 || n[0] != (SectorCoord{-1, 0}) {
		t.Fatalf("ghost oeste: %v", n)
	}
	if n := NeighborsForGhost(Vec2{2040, 2040}); len(n) != 3 {
		t.Fatalf("ghost canto: %v", n)
	}
	all := []Entity{
		{ID: 1, OwnerID: "me", Pos: Vec2{0, 0}, Vision: 300},
		{ID: 2, OwnerID: "enemy", Pos: Vec2{200, 0}},
		{ID: 3, OwnerID: "enemy", Pos: Vec2{900, 0}},
		{ID: 4, OwnerID: "ally", Pos: Vec2{1000, 0}, Vision: 300},
	}
	if v := VisibleTo("me", nil, all); len(v) != 2 {
		t.Fatalf("sem aliado: %d", len(v))
	}
	if v := VisibleTo("me", map[string]bool{"ally": true}, all); len(v) != 4 {
		t.Fatalf("com visão do aliado: %d", len(v))
	}
}
