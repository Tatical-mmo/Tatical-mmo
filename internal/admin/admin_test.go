package admin

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/tactical-mmo/internal/economy"
	"github.com/seu-usuario/tactical-mmo/internal/moderation"
	"github.com/seu-usuario/tactical-mmo/internal/rbac"
)

const adminID = "acc:gabriel"

func newSvc() *Service {
	l := economy.NewLedger()
	return &Service{
		SupremeAdminID: adminID,
		Ledger:         l,
		Shop:           economy.NewShop(l, []economy.Cosmetic{{ID: "camo_arctic", Price: 1200}}),
		Sanctions:      moderation.NewStore(),
		Roles:          NewRoleStore(),
		Audit:          &AuditLog{},
	}
}

func TestAdminInfiniteResourcesAndDonation(t *testing.T) {
	s := newSvc()
	if err := s.GrantResources(adminID, adminID, economy.Steel, 1_000_000_000, "r1"); err != nil {
		t.Fatal(err)
	}
	// Doação maior que o saldo: emite a diferença automaticamente.
	if err := s.DonateResources(adminID, "acc:p1", economy.Steel, 1_500_000_000, "r2"); err != nil {
		t.Fatal(err)
	}
	if got := s.Ledger.Balance("acc:p1", economy.Steel); got != 1_500_000_000 {
		t.Fatalf("p1 = %d", got)
	}
	if got := s.Ledger.MintedByAdmin(economy.Steel); got != 1_500_000_000 {
		t.Fatalf("minted = %d", got)
	}
	// Repetição com o mesmo requestID é idempotente.
	_ = s.GrantResources(adminID, adminID, economy.Steel, 1_000_000_000, "r1")
	if got := s.Ledger.MintedByAdmin(economy.Steel); got != 1_500_000_000 {
		t.Fatalf("idempotência falhou: %d", got)
	}
}

func TestPlayerCannotGrant(t *testing.T) {
	s := newSvc()
	err := s.GrantResources("acc:p1", "acc:p1", economy.Credits, 10, "x")
	if !errors.Is(err, rbac.ErrForbidden) {
		t.Fatalf("esperava negação, veio %v", err)
	}
	recs := s.Audit.Records()
	if len(recs) != 1 || recs[0].Outcome == "OK" {
		t.Fatal("tentativa negada deve ficar na auditoria")
	}
}

func TestShopFreeForAdminPaidForPlayer(t *testing.T) {
	s := newSvc()
	p, err := s.BuyCosmetic(adminID, "camo_arctic")
	if err != nil || p.PricePaid != 0 || !p.AdminFree {
		t.Fatalf("admin: %+v %v", p, err)
	}
	if _, err := s.BuyCosmetic("acc:p1", "camo_arctic"); !errors.Is(err, economy.ErrInsufficient) {
		t.Fatalf("jogador sem saldo deveria falhar: %v", err)
	}
}

func TestModeratorRules(t *testing.T) {
	s := newSvc()
	if err := s.GrantRole(adminID, "acc:mod", rbac.PermChatMute|rbac.PermFreeze); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantRole(adminID, "acc:mod", rbac.PermBanPerm); !errors.Is(err, rbac.ErrAdminOnly) {
		t.Fatalf("BAN_PERM não pode ir para moderador: %v", err)
	}
	if err := s.Sanction("acc:mod", "acc:cheater", moderation.Freeze, time.Hour, "speedhack", ""); err != nil {
		t.Fatal(err)
	}
	if !s.Sanctions.IsFrozen("acc:cheater") {
		t.Fatal("deveria estar congelado")
	}
	if err := s.Sanction("acc:mod", "acc:cheater", moderation.BanTemp, time.Hour, "x", ""); !errors.Is(err, rbac.ErrForbidden) {
		t.Fatalf("mod sem BAN_TEMP: %v", err)
	}
	if err := s.Sanction("acc:mod", adminID, moderation.Mute, time.Hour, "x", ""); !errors.Is(err, rbac.ErrTargetOutranks) {
		t.Fatalf("mod não age sobre admin: %v", err)
	}
	if err := s.GrantRole("acc:mod", "acc:p2", rbac.PermChatMute); !errors.Is(err, rbac.ErrForbidden) {
		t.Fatalf("mod não concede cargos: %v", err)
	}
	if err := s.Sanction(adminID, "acc:cheater", moderation.BanPerm, 0, "reincidente", "replay#42"); err != nil {
		t.Fatal(err)
	}
	if !s.Sanctions.IsBanned("acc:cheater") {
		t.Fatal("deveria estar banido")
	}
	if err := s.RevokeRole(adminID, "acc:mod"); err != nil || s.Roles.Get("acc:mod") != 0 {
		t.Fatal("revogação falhou")
	}
}

func TestAutoAction(t *testing.T) {
	cases := []struct {
		score int
		sig   bool
		prior int
		want  moderation.AutoAction
	}{
		{10, false, 0, moderation.ActNone},
		{45, false, 0, moderation.ActReviewQueue},
		{75, false, 0, moderation.ActAutoFreeze},
		{95, false, 0, moderation.ActAutoBanTemp},
		{20, true, 1, moderation.ActProposePermBan},
	}
	for _, c := range cases {
		if got := moderation.DecideAuto(c.score, c.sig, c.prior); got != c.want {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}
