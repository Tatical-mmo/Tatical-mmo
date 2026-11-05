// Package world define a grade de setores/chunks e o interest management
// (o que cada jogador recebe). Cada setor é simulado por um shard.
package world

import "math"

const (
	SectorSize    = 2048.0 // unidades de mundo por lado
	ChunkSize     = 64.0
	ChunksPerAxis = int(SectorSize / ChunkSize) // 32
	GhostMargin   = 128.0                       // entidades perto da borda são replicadas no vizinho
)

type SectorCoord struct{ X, Y int }
type ChunkCoord struct{ X, Y int }

type Vec2 struct{ X, Y float64 }

func (a Vec2) Dist(b Vec2) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// SectorOf converte posição global em setor.
func SectorOf(p Vec2) SectorCoord {
	return SectorCoord{int(math.Floor(p.X / SectorSize)), int(math.Floor(p.Y / SectorSize))}
}

func ChunkOf(p Vec2) ChunkCoord {
	return ChunkCoord{int(math.Floor(p.X / ChunkSize)), int(math.Floor(p.Y / ChunkSize))}
}

// NeighborsForGhost devolve os setores vizinhos que devem receber uma cópia
// "fantasma" somente-leitura da entidade (está a menos de GhostMargin da borda).
func NeighborsForGhost(p Vec2) []SectorCoord {
	s := SectorOf(p)
	lx, ly := p.X-float64(s.X)*SectorSize, p.Y-float64(s.Y)*SectorSize
	dx, dy := 0, 0
	if lx < GhostMargin {
		dx = -1
	} else if lx > SectorSize-GhostMargin {
		dx = 1
	}
	if ly < GhostMargin {
		dy = -1
	} else if ly > SectorSize-GhostMargin {
		dy = 1
	}
	var out []SectorCoord
	if dx != 0 {
		out = append(out, SectorCoord{s.X + dx, s.Y})
	}
	if dy != 0 {
		out = append(out, SectorCoord{s.X, s.Y + dy})
	}
	if dx != 0 && dy != 0 {
		out = append(out, SectorCoord{s.X + dx, s.Y + dy})
	}
	return out
}

type SectorType string

type Sector struct {
	Coord   SectorCoord
	Type    SectorType // "plateau", "desert", "urban", "port"
	Seed    int64
	OwnerID string // jogador que controla o ponto de dominação central ("" = neutro)
	ShardID string // processo que simula este setor (World Directory)
	Active  bool
}

// Directory é o World Directory: registro de setores e expansão por anéis.
type Directory struct {
	Sectors map[SectorCoord]*Sector
	Radius  int // anel atual: setores com |x|,|y| <= Radius
}

func NewDirectory(radius int, typeFor func(SectorCoord) SectorType) *Directory {
	d := &Directory{Sectors: map[SectorCoord]*Sector{}}
	for r := 0; r <= radius; r++ {
		d.addRing(r, typeFor)
	}
	d.Radius = radius
	return d
}

// ExpandRing abre um novo anel de setores neutros (comando WORLD_CONTROL do Admin).
func (d *Directory) ExpandRing(typeFor func(SectorCoord) SectorType) []SectorCoord {
	d.Radius++
	return d.addRing(d.Radius, typeFor)
}

func (d *Directory) addRing(r int, typeFor func(SectorCoord) SectorType) []SectorCoord {
	var added []SectorCoord
	for x := -r; x <= r; x++ {
		for y := -r; y <= r; y++ {
			if max(abs(x), abs(y)) != r {
				continue
			}
			c := SectorCoord{x, y}
			if _, ok := d.Sectors[c]; !ok {
				d.Sectors[c] = &Sector{Coord: c, Type: typeFor(c), Seed: int64(x*73856093 ^ y*19349663), Active: true}
				added = append(added, c)
			}
		}
	}
	return added
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------- Interest management ----------------

type Entity struct {
	ID      uint32
	OwnerID string
	Pos     Vec2
	Vision  float64
	SkinID  string // repassado ao cliente; o combate nunca lê este campo
}

// VisibleTo devolve as entidades que 'viewer' pode ver: as suas, as dos aliados
// com visão compartilhada, e as inimigas dentro do raio de visão de alguma
// unidade sua/aliada. O resto NUNCA é enviado (isso também impede maphack).
func VisibleTo(viewer string, allies map[string]bool, all []Entity) []Entity {
	var eyes []Entity
	for _, e := range all {
		if e.OwnerID == viewer || allies[e.OwnerID] {
			eyes = append(eyes, e)
		}
	}
	var out []Entity
	for _, e := range all {
		if e.OwnerID == viewer || allies[e.OwnerID] {
			out = append(out, e)
			continue
		}
		for _, eye := range eyes {
			if eye.Pos.Dist(e.Pos) <= eye.Vision {
				out = append(out, e)
				break
			}
		}
	}
	return out
}
