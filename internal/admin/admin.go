// Package admin é a ÚNICA porta para comandos privilegiados (Admin e Moderadores).
// Fluxo: autoriza -> grava auditoria (antes de executar) -> executa -> grava resultado.
package admin

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/seu-usuario/tactical-mmo/internal/economy"
	"github.com/seu-usuario/tactical-mmo/internal/moderation"
	"github.com/seu-usuario/tactical-mmo/internal/rbac"
)

type AuditRecord struct {
	ID      int64
	ActorID string
	Command string
	Target  string
	Payload string
	At      time.Time
	Outcome string // "PENDING", "OK" ou a mensagem de erro
}

// AuditLog é append-only. Em produção: tabela audit_log sem UPDATE/DELETE para o
// usuário do banco da aplicação (só INSERT), mais cópia para armazenamento externo.
type AuditLog struct {
	mu   sync.Mutex
	recs []AuditRecord
}

func (a *AuditLog) begin(r AuditRecord) int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	r.ID, r.At, r.Outcome = int64(len(a.recs)+1), time.Now(), "PENDING"
	a.recs = append(a.recs, r)
	return r.ID
}

func (a *AuditLog) finish(id int64, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.recs[id-1].Outcome = err.Error()
	} else {
		a.recs[id-1].Outcome = "OK"
	}
}

func (a *AuditLog) Records() []AuditRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]AuditRecord(nil), a.recs...)
}

// RoleStore guarda as permissões de moderadores (o Admin não está aqui).
type RoleStore struct {
	mu    sync.Mutex
	perms map[string]rbac.Permission
}

func NewRoleStore() *RoleStore { return &RoleStore{perms: map[string]rbac.Permission{}} }

func (r *RoleStore) Get(id string) rbac.Permission {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.perms[id]
}

// SessionKiller derruba sessões ativas (implementado pelo Gateway via NATS).
type SessionKiller interface {
	Disconnect(accountID, reason string)
}

type Service struct {
	SupremeAdminID string // vem da config do servidor
	Ledger         *economy.Ledger
	Shop           *economy.Shop
	Sanctions      *moderation.Store
	Roles          *RoleStore
	Audit          *AuditLog
	Sessions       SessionKiller
}

func (s *Service) principal(id string) rbac.Principal {
	return rbac.ResolvePrincipal(id, s.SupremeAdminID, s.Roles.Get(id))
}

func (s *Service) rankOf(id string) rbac.Rank { return s.principal(id).Rank }

// run envolve todo comando: permissão, auditoria antes e depois.
func (s *Service) run(actorID, cmd, target, payload string, need rbac.Permission, fn func(p rbac.Principal) error) error {
	p := s.principal(actorID)
	id := s.Audit.begin(AuditRecord{ActorID: actorID, Command: cmd, Target: target, Payload: payload})
	err := rbac.Require(p, need)
	if err == nil {
		err = fn(p)
	}
	s.Audit.finish(id, err)
	return err
}

// ---------------- Economia ----------------

func (s *Service) GrantResources(actorID, targetID string, r economy.Resource, amount int64, reqID string) error {
	return s.run(actorID, "GrantResources", targetID, fmt.Sprintf("%s=%d", r, amount), rbac.PermEconomyGrant,
		func(rbac.Principal) error {
			_, err := s.Ledger.Transfer("admin:"+reqID, economy.KindAdminMint, economy.AccMint, targetID, r, amount, actorID)
			return err
		})
}

func (s *Service) DonateResources(actorID, targetID string, r economy.Resource, amount int64, reqID string) error {
	return s.run(actorID, "DonateResources", targetID, fmt.Sprintf("%s=%d", r, amount), rbac.PermDonate,
		func(rbac.Principal) error {
			_, err := s.Ledger.MintAndTransfer("admin:"+reqID, actorID, targetID, r, amount, actorID)
			return err
		})
}

// BuyCosmetic é chamado pela loja para QUALQUER jogador; o Admin simplesmente recebe preço 0.
func (s *Service) BuyCosmetic(actorID, cosmeticID string) (economy.Purchase, error) {
	p := s.principal(actorID)
	return s.Shop.Buy(actorID, cosmeticID, p.Perms.Has(rbac.PermShopFree))
}

// ---------------- Moderação ----------------

var ErrDurationTooLong = errors.New("admin: duração acima do limite do cargo")

func (s *Service) Sanction(actorID, targetID string, t moderation.SanctionType, d time.Duration, reason, evidence string) error {
	need := map[moderation.SanctionType]rbac.Permission{
		moderation.Mute: rbac.PermChatMute, moderation.Kick: rbac.PermKick, moderation.Freeze: rbac.PermFreeze,
		moderation.BanTemp: rbac.PermBanTemp, moderation.BanPerm: rbac.PermBanPerm,
	}[t]
	return s.run(actorID, "Sanction:"+string(t), targetID, reason, need, func(p rbac.Principal) error {
		if err := rbac.CanActOn(p, s.rankOf(targetID)); err != nil {
			return err
		}
		if p.Rank != rbac.RankSupremeAdmin &&
			((t == moderation.Mute && d > moderation.MaxModMute) || (t == moderation.BanTemp && d > moderation.MaxModBanTemp)) {
			return ErrDurationTooLong
		}
		sn := moderation.Sanction{TargetID: targetID, Type: t, Reason: reason, Evidence: evidence, IssuedBy: actorID}
		if t != moderation.BanPerm && t != moderation.Kick {
			exp := time.Now().Add(d)
			sn.ExpiresAt = &exp
		}
		if _, err := s.Sanctions.Issue(sn); err != nil {
			return err
		}
		if (t == moderation.Kick || t == moderation.BanTemp || t == moderation.BanPerm) && s.Sessions != nil {
			s.Sessions.Disconnect(targetID, reason)
		}
		return nil
	})
}

// ---------------- Cargos ----------------

func (s *Service) GrantRole(actorID, targetID string, perms rbac.Permission) error {
	return s.run(actorID, "GrantRole", targetID, fmt.Sprintf("perms=%d", perms), rbac.PermRoleManage,
		func(p rbac.Principal) error {
			if err := rbac.ValidateGrant(p, perms); err != nil {
				return err
			}
			if targetID == s.SupremeAdminID {
				return rbac.ErrTargetOutranks
			}
			s.Roles.mu.Lock()
			s.Roles.perms[targetID] |= perms
			s.Roles.mu.Unlock()
			return nil
		})
}

func (s *Service) RevokeRole(actorID, targetID string) error {
	return s.run(actorID, "RevokeRole", targetID, "", rbac.PermRoleManage, func(rbac.Principal) error {
		s.Roles.mu.Lock()
		delete(s.Roles.perms, targetID)
		s.Roles.mu.Unlock()
		if s.Sessions != nil { // força relogin sem os poderes antigos
			s.Sessions.Disconnect(targetID, "cargo revogado")
		}
		return nil
	})
}
