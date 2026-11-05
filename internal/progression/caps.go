// Package progression calcula o teto de nível efetivo (trava tripla) e a curva de custo.
package progression

import (
	"errors"
	"math"
	"time"
)

const MaxTier = 5

// LevelCapForTier: T1=5, T2=10 ... T5=25 (QG, outras construções e tropas).
func LevelCapForTier(tier int) int { return tier * 5 }

// SeasonSchedule: semana (a partir de 1) em que cada tier é liberado no servidor.
var SeasonSchedule = map[int]int{1: 1, 2: 1, 3: 3, 4: 5, 5: 7}

// ServerTier é o tier máximo liberado pelo calendário da temporada.
// override > 0 é o ajuste manual do Admin (WORLD_CONTROL).
func ServerTier(seasonStart, now time.Time, override int) int {
	if override > 0 {
		return min(override, MaxTier)
	}
	week := int(now.Sub(seasonStart).Hours()/(24*7)) + 1
	tier := 0
	for t := 1; t <= MaxTier; t++ {
		if week >= SeasonSchedule[t] {
			tier = t
		}
	}
	return tier
}

type Kind int

const (
	HQ Kind = iota
	Building
	Troop
)

type PlayerState struct {
	ResearchedTier int // tier pesquisado (exige setor do tipo certo — checado em outro lugar)
	HQLevel        int
}

// MaxAllowedLevel = menor das três travas.
func MaxAllowedLevel(k Kind, p PlayerState, serverTier int) int {
	tier := min(p.ResearchedTier, serverTier)
	limit := LevelCapForTier(tier)
	if k != HQ {
		limit = min(limit, p.HQLevel)
	}
	return limit
}

var ErrAboveCap = errors.New("progression: nível acima do teto atual")

func CanUpgrade(k Kind, current int, p PlayerState, serverTier int) error {
	if current+1 > MaxAllowedLevel(k, p, serverTier) {
		return ErrAboveCap
	}
	return nil
}

// Cost: base × 1,6^(L-1). Desconto de 25% para quem está 2+ tiers atrás do servidor.
func Cost(base int64, level int, p PlayerState, serverTier int) int64 {
	c := float64(base) * math.Pow(1.6, float64(level-1))
	if serverTier-p.ResearchedTier >= 2 {
		c *= 0.75
	}
	return int64(math.Round(c))
}

func BuildTime(base time.Duration, level int) time.Duration {
	return time.Duration(float64(base) * math.Pow(1.45, float64(level-1)))
}
