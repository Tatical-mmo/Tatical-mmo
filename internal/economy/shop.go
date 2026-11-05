package economy

import (
	"errors"
	"fmt"
	"sync"
)

// Cosmetic deliberadamente NÃO tem campos de atributo de combate.
// O motor de combate nunca importa este tipo — só o renderizador do cliente usa SkinID.
type Cosmetic struct {
	ID       string
	Name     string
	Slot     string // "tank_camo", "uniform", "commander"
	Price    int64  // em Créditos
	AssetRef string // caminho do asset no cliente
}

type Purchase struct {
	AccountID  string
	CosmeticID string
	PricePaid  int64
	AdminFree  bool
}

var ErrAlreadyOwned = errors.New("shop: item já possuído")

type Shop struct {
	mu        sync.Mutex
	ledger    *Ledger
	catalog   map[string]Cosmetic
	inventory map[string]map[string]bool
	purchases []Purchase
}

func NewShop(l *Ledger, items []Cosmetic) *Shop {
	s := &Shop{ledger: l, catalog: map[string]Cosmetic{}, inventory: map[string]map[string]bool{}}
	for _, it := range items {
		s.catalog[it.ID] = it
	}
	return s
}

// Buy aplica a regra de preço: freeShop=true (permissão SHOP_FREE) zera o custo.
func (s *Shop) Buy(accountID, cosmeticID string, freeShop bool) (Purchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.catalog[cosmeticID]
	if !ok {
		return Purchase{}, fmt.Errorf("shop: item %q não existe", cosmeticID)
	}
	if s.inventory[accountID][cosmeticID] {
		return Purchase{}, ErrAlreadyOwned
	}
	price := item.Price
	if freeShop {
		price = 0
	}
	if price > 0 {
		key := "shop:" + accountID + ":" + cosmeticID
		if _, err := s.ledger.Transfer(key, KindShopPurchase, accountID, AccBurn, Credits, price, accountID); err != nil {
			return Purchase{}, err
		}
	}
	if s.inventory[accountID] == nil {
		s.inventory[accountID] = map[string]bool{}
	}
	s.inventory[accountID][cosmeticID] = true
	p := Purchase{AccountID: accountID, CosmeticID: cosmeticID, PricePaid: price, AdminFree: freeShop}
	s.purchases = append(s.purchases, p)
	return p, nil
}

func (s *Shop) Owns(accountID, cosmeticID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inventory[accountID][cosmeticID]
}
