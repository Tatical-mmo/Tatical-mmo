# Tactical MMO — código inicial

Esqueleto do backend (Go 1.24, só biblioteca padrão) com as regras do documento de arquitetura.

```
internal/rbac         permissões em bitmask, Admin vem da config, regras de concessão
internal/admin        porta única de comandos privilegiados + auditoria
internal/economy      ledger de dupla entrada, emissão/doação, loja cosmética
internal/moderation   sanções e ações automáticas do anti-cheat
internal/progression  teto de nível (trava tripla), tiers da temporada, custos
internal/social       alianças, traição, marca de Traidor
internal/world        setores, chunks, expansão por anéis, visibilidade
db/schema.sql         schema PostgreSQL (testado no PG 16)
proto/game.proto      protocolo cliente/servidor + AdminService
```

## Rodar os testes

```
go test ./...
```

## Configuração do Admin

O Admin Supremo é definido por variável de ambiente no servidor, nunca no banco:

```
SUPREME_ADMIN_ID=acc:seu-id
```

## Próximos passos

1. Trocar o `Ledger` em memória por uma implementação Postgres (`ledger_entries` + `wallets` com `SELECT ... FOR UPDATE`).
2. Gerar o código do `game.proto` (`protoc --go_out=. proto/game.proto`) e ligar o Gateway WebSocket.
3. Escrever o loop do shard (20 Hz) usando `world.VisibleTo` para montar os `WorldDelta`.
