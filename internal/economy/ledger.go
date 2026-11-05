// Package economy implementa carteiras sobre um ledger de dupla entrada.
// Nenhum saldo é editado diretamente: toda mudança é um lançamento débito/crédito.
package economy

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

type Resource string

const (
	Steel   Resource = "STEEL"   // Aço
	Fuel    Resource = "FUEL"    // Combustível
	Energy  Resource = "ENERGY"  // Energia
	Credits Resource = "CREDITS" // Créditos (loja cosmética, persiste entre temporadas)
)

// Contas de sistema.
const (
	AccMint     = "sys:mint"     // emissão (fonte de todo valor novo)
	AccBurn     = "sys:burn"     // queima (sumidouros)
	AccTreasury = "sys:treasury" // tesouraria
)

type EntryKind string

const (
	KindDominationIncome EntryKind = "DOMINATION_INCOME"
	KindSalvage          EntryKind = "SALVAGE"
	KindObjective        EntryKind = "OBJECTIVE"
	KindSpend            EntryKind = "SPEND"
	KindShopPurchase     EntryKind = "SHOP_PURCHASE"
	KindAdminMint        EntryKind = "ADMIN_MINT"
	KindAdminDonation    EntryKind = "ADMIN_DONATION"
	KindReversal         EntryKind = "REVERSAL"
)

type Entry struct {
	ID             int64
	IdempotencyKey string
	Kind           EntryKind
	From, To       string
	Resource       Resource
	Amount         int64
	ActorID        string // quem originou (jogador, sistema ou admin)
	At             time.Time
}

var (
	ErrInsufficient = errors.New("economy: saldo insuficiente")
	ErrOverflow     = errors.New("economy: overflow de saldo")
	ErrBadAmount    = errors.New("economy: valor deve ser positivo")
)

// Ledger em memória. Em produção: tabela ledger_entries no Postgres, com
// SELECT ... FOR UPDATE nas carteiras dentro de uma transação.
type Ledger struct {
	mu       sync.Mutex
	entries  []Entry
	balances map[string]map[Resource]int64
	seen     map[string]int64 // idempotencyKey -> entry ID
	now      func() time.Time
}

func NewLedger() *Ledger {
	return &Ledger{balances: map[string]map[Resource]int64{}, seen: map[string]int64{}, now: time.Now}
}

func isSystem(acc string) bool { return acc == AccMint || acc == AccBurn || acc == AccTreasury }

func (l *Ledger) Balance(acc string, r Resource) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.balances[acc][r]
}

// Transfer é a única primitiva de escrita. Contas de sistema podem ficar negativas
// (AccMint fica negativa na medida exata de tudo que foi emitido).
func (l *Ledger) Transfer(key string, kind EntryKind, from, to string, r Resource, amount int64, actor string) (Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.transferLocked(key, kind, from, to, r, amount, actor)
}

func (l *Ledger) transferLocked(key string, kind EntryKind, from, to string, r Resource, amount int64, actor string) (Entry, error) {
	if amount <= 0 {
		return Entry{}, ErrBadAmount
	}
	if id, ok := l.seen[key]; ok && key != "" {
		return l.entries[id-1], nil // repetição idempotente
	}
	if !isSystem(from) && l.balances[from][r] < amount {
		return Entry{}, fmt.Errorf("%w: %s tem %d %s, precisa de %d", ErrInsufficient, from, l.balances[from][r], r, amount)
	}
	if l.balances[to][r] > math.MaxInt64-amount {
		return Entry{}, ErrOverflow
	}
	for _, acc := range []string{from, to} {
		if l.balances[acc] == nil {
			l.balances[acc] = map[Resource]int64{}
		}
	}
	l.balances[from][r] -= amount
	l.balances[to][r] += amount
	e := Entry{ID: int64(len(l.entries) + 1), IdempotencyKey: key, Kind: kind, From: from, To: to,
		Resource: r, Amount: amount, ActorID: actor, At: l.now()}
	l.entries = append(l.entries, e)
	if key != "" {
		l.seen[key] = e.ID
	}
	return e, nil
}

// MintAndTransfer emite o que faltar na carteira 'from' e transfere, numa operação atômica.
// Usado na doação do Admin quando o saldo dele não basta.
func (l *Ledger) MintAndTransfer(key string, from, to string, r Resource, amount int64, actor string) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Entry
	if missing := amount - l.balances[from][r]; missing > 0 {
		e, err := l.transferLocked(key+":mint", KindAdminMint, AccMint, from, r, missing, actor)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	e, err := l.transferLocked(key, KindAdminDonation, from, to, r, amount, actor)
	if err != nil {
		return out, err
	}
	return append(out, e), nil
}

// MintedByAdmin soma tudo emitido por ADMIN_MINT — usado na métrica de inflação.
func (l *Ledger) MintedByAdmin(r Resource) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var sum int64
	for _, e := range l.entries {
		if e.Kind == KindAdminMint && e.Resource == r {
			sum += e.Amount
		}
	}
	return sum
}

func (l *Ledger) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Entry(nil), l.entries...)
}
