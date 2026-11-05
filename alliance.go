// Package social implementa alianças temporárias e a traição em tempo real.
package social

import (
	"errors"
	"time"
)

const (
	MaxMembers          = 3
	FriendlyFireGrace   = 3 * time.Second // aviso antes do fogo amigo ligar
	TraitorMarkDuration = 30 * time.Minute
)

type State string

const (
	Proposed State = "PROPOSED"
	Active   State = "ACTIVE"
	Broken   State = "BROKEN"
	Expired  State = "EXPIRED"
)

type Alliance struct {
	ID        string
	Members   []string
	State     State
	ExpiresAt time.Time
	BrokenBy  string
	BrokenAt  time.Time
}

var (
	ErrFull      = errors.New("alliance: limite de membros")
	ErrNotMember = errors.New("alliance: não é membro")
	ErrNotActive = errors.New("alliance: pacto não está ativo")
)

func (a *Alliance) has(id string) bool {
	for _, m := range a.Members {
		if m == id {
			return true
		}
	}
	return false
}

func (a *Alliance) Join(id string) error {
	if len(a.Members) >= MaxMembers {
		return ErrFull
	}
	if !a.has(id) {
		a.Members = append(a.Members, id)
	}
	if len(a.Members) >= 2 {
		a.State = Active
	}
	return nil
}

// Betray rompe o pacto. Efeitos aplicados pelo shard:
//   - visão compartilhada cortada NO MESMO TICK
//   - fogo amigo liga em now+FriendlyFireGrace
//   - marca "Traidor" no traidor por TraitorMarkDuration
func (a *Alliance) Betray(traitor string, now time.Time) error {
	if a.State != Active {
		return ErrNotActive
	}
	if !a.has(traitor) {
		return ErrNotMember
	}
	a.State, a.BrokenBy, a.BrokenAt = Broken, traitor, now
	return nil
}

// Hostile diz se 'x' pode causar dano em 'y' no instante 'now'.
func Hostile(a *Alliance, x, y string, now time.Time) bool {
	if a == nil || !a.has(x) || !a.has(y) {
		return true
	}
	switch a.State {
	case Active:
		return now.After(a.ExpiresAt) && !a.ExpiresAt.IsZero()
	case Broken:
		return !now.Before(a.BrokenAt.Add(FriendlyFireGrace))
	}
	return true
}

// SharesVision é falso imediatamente após a quebra.
func SharesVision(a *Alliance, x, y string, now time.Time) bool {
	return a != nil && a.State == Active && a.has(x) && a.has(y) &&
		(a.ExpiresAt.IsZero() || now.Before(a.ExpiresAt))
}

func IsTraitorMarked(a *Alliance, id string, now time.Time) bool {
	return a != nil && a.State == Broken && a.BrokenBy == id && now.Before(a.BrokenAt.Add(TraitorMarkDuration))
}
