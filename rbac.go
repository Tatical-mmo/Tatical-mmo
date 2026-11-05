// Package rbac define permissões atômicas, cargos e as regras de quem pode
// conceder o quê. O SUPREME_ADMIN vem da configuração do servidor, nunca do banco.
package rbac

import (
	"errors"
	"fmt"
)

type Permission uint64

const (
	PermChatMute Permission = 1 << iota
	PermKick
	PermFreeze
	PermSpectate
	PermBanTemp
	PermViewReports
	// --- Apenas Admin a partir daqui ---
	PermBanPerm
	PermEconomyGrant
	PermShopFree
	PermDonate
	PermRoleManage
	PermWorldControl
)

// AdminOnly nunca pode entrar num cargo de moderador.
const AdminOnly = PermBanPerm | PermEconomyGrant | PermShopFree | PermDonate | PermRoleManage | PermWorldControl

// ModeratorAssignable é tudo que um moderador pode receber.
const ModeratorAssignable = PermChatMute | PermKick | PermFreeze | PermSpectate | PermBanTemp | PermViewReports

const All = AdminOnly | ModeratorAssignable

var names = map[Permission]string{
	PermChatMute: "CHAT_MUTE", PermKick: "KICK", PermFreeze: "FREEZE",
	PermSpectate: "SPECTATE", PermBanTemp: "BAN_TEMP", PermViewReports: "VIEW_REPORTS",
	PermBanPerm: "BAN_PERM", PermEconomyGrant: "ECONOMY_GRANT", PermShopFree: "SHOP_FREE",
	PermDonate: "DONATE", PermRoleManage: "ROLE_MANAGE", PermWorldControl: "WORLD_CONTROL",
}

func (p Permission) String() string {
	if n, ok := names[p]; ok {
		return n
	}
	return fmt.Sprintf("PERM(%d)", uint64(p))
}

func (p Permission) Has(q Permission) bool { return p&q == q }

type Rank int

const (
	RankPlayer Rank = iota
	RankModerator
	RankSupremeAdmin
)

// Principal é quem está executando um comando.
type Principal struct {
	AccountID string
	Rank      Rank
	Perms     Permission
}

// ResolvePrincipal monta o Principal. supremeAdminID vem da config do servidor.
func ResolvePrincipal(accountID, supremeAdminID string, modPerms Permission) Principal {
	if accountID == supremeAdminID {
		return Principal{AccountID: accountID, Rank: RankSupremeAdmin, Perms: All}
	}
	if modPerms != 0 {
		return Principal{AccountID: accountID, Rank: RankModerator, Perms: modPerms & ModeratorAssignable}
	}
	return Principal{AccountID: accountID, Rank: RankPlayer}
}

var (
	ErrForbidden      = errors.New("rbac: permissão negada")
	ErrAdminOnly      = errors.New("rbac: permissão exclusiva do Admin não pode ir para moderador")
	ErrTargetOutranks = errors.New("rbac: alvo tem cargo igual ou superior")
)

// Require falha se o principal não tiver a permissão.
func Require(p Principal, perm Permission) error {
	if !p.Perms.Has(perm) {
		return fmt.Errorf("%w: precisa de %s", ErrForbidden, perm)
	}
	return nil
}

// CanActOn: moderador não age sobre moderador nem Admin; ninguém age sobre o Admin.
func CanActOn(actor Principal, targetRank Rank) error {
	if targetRank == RankSupremeAdmin || (actor.Rank != RankSupremeAdmin && targetRank >= actor.Rank) {
		return ErrTargetOutranks
	}
	return nil
}

// ValidateGrant aplica as regras: só quem tem ROLE_MANAGE concede, ninguém concede
// o que não tem, e permissões AdminOnly nunca vão para moderadores.
func ValidateGrant(actor Principal, perms Permission) error {
	if err := Require(actor, PermRoleManage); err != nil {
		return err
	}
	if perms&AdminOnly != 0 {
		return ErrAdminOnly
	}
	if !actor.Perms.Has(perms) {
		return ErrForbidden
	}
	return nil
}
