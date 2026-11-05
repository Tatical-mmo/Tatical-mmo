// Package moderation guarda sanções e o estado de punição de cada conta,
// e converte pontuação de suspeita do anti-cheat em ações automáticas.
package moderation

import (
	"errors"
	"sync"
	"time"
)

type SanctionType string

const (
	Mute    SanctionType = "MUTE"
	Kick    SanctionType = "KICK"
	Freeze  SanctionType = "FREEZE"
	BanTemp SanctionType = "BAN_TEMP"
	BanPerm SanctionType = "BAN_PERM"
)

const (
	MaxModMute    = 24 * time.Hour
	MaxModBanTemp = 7 * 24 * time.Hour
)

type Sanction struct {
	ID        int64
	TargetID  string
	Type      SanctionType
	Reason    string
	Evidence  string
	IssuedBy  string // ID da conta, ou "system:anticheat"
	Automatic bool
	IssuedAt  time.Time
	ExpiresAt *time.Time // nil = permanente (ou instantâneo, para KICK)
	RevokedAt *time.Time
}

var ErrReasonRequired = errors.New("moderation: motivo obrigatório")

type Store struct {
	mu        sync.Mutex
	sanctions []Sanction
	now       func() time.Time
}

func NewStore() *Store { return &Store{now: time.Now} }

func (s *Store) Issue(sn Sanction) (Sanction, error) {
	if sn.Reason == "" {
		return Sanction{}, ErrReasonRequired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sn.ID = int64(len(s.sanctions) + 1)
	sn.IssuedAt = s.now()
	s.sanctions = append(s.sanctions, sn)
	return sn, nil
}

func (s *Store) active(target string, t SanctionType) bool {
	now := s.now()
	for _, sn := range s.sanctions {
		if sn.TargetID == target && sn.Type == t && sn.RevokedAt == nil &&
			(sn.ExpiresAt == nil || sn.ExpiresAt.After(now)) {
			return true
		}
	}
	return false
}

// IsBanned é consultado pelo Gateway em todo login e reconexão.
func (s *Store) IsBanned(target string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(target, BanPerm) || s.active(target, BanTemp)
}

func (s *Store) IsFrozen(target string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(target, Freeze)
}

func (s *Store) PriorBans(target string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, sn := range s.sanctions {
		if sn.TargetID == target && (sn.Type == BanTemp || sn.Type == BanPerm) {
			n++
		}
	}
	return n
}

// AutoAction é o que o anti-cheat decide para uma pontuação.
type AutoAction int

const (
	ActNone AutoAction = iota
	ActReviewQueue
	ActAutoFreeze
	ActAutoBanTemp
	ActProposePermBan // nunca aplicado sozinho: vai para confirmação do Admin
)

// DecideAuto implementa a tabela de limiares do documento.
func DecideAuto(score int, confirmedSignature bool, priorBans int) AutoAction {
	switch {
	case (score >= 90 || confirmedSignature) && priorBans > 0:
		return ActProposePermBan
	case score >= 90 || confirmedSignature:
		return ActAutoBanTemp
	case score >= 70:
		return ActAutoFreeze
	case score >= 40:
		return ActReviewQueue
	}
	return ActNone
}
