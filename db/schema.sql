-- Schema PostgreSQL — Jogo de Estratégia Tática MMO
-- O SUPREME_ADMIN NÃO está aqui: é definido na config do servidor (SUPREME_ADMIN_ID).

CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    account_level INT  NOT NULL DEFAULT 1,
    show_gm_badge BOOLEAN NOT NULL DEFAULT FALSE,
    exclude_from_rankings BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE devices (
    account_id  TEXT REFERENCES accounts(id),
    fingerprint TEXT NOT NULL,
    last_ip     INET,
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, fingerprint)
);

-- ---------------- RBAC ----------------
CREATE TABLE role_templates (            -- "Moderador de Chat", "Moderador Sênior"...
    id          SERIAL PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    permissions BIGINT NOT NULL           -- bitmask (ver internal/rbac)
);

CREATE TABLE moderator_grants (
    account_id  TEXT PRIMARY KEY REFERENCES accounts(id),
    permissions BIGINT NOT NULL,
    template_id INT REFERENCES role_templates(id),
    granted_by  TEXT NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Bits AdminOnly (BAN_PERM e acima: 1<<6 .. 1<<11) proibidos para moderadores
    CONSTRAINT no_admin_only_bits CHECK ((permissions & 4032) = 0)
);

-- ---------------- Moderação ----------------
CREATE TYPE sanction_type AS ENUM ('MUTE','KICK','FREEZE','BAN_TEMP','BAN_PERM');

CREATE TABLE sanctions (
    id          BIGSERIAL PRIMARY KEY,
    target_id   TEXT NOT NULL REFERENCES accounts(id),
    type        sanction_type NOT NULL,
    reason      TEXT NOT NULL CHECK (reason <> ''),
    evidence    TEXT,
    issued_by   TEXT NOT NULL,           -- account id ou 'system:anticheat'
    automatic   BOOLEAN NOT NULL DEFAULT FALSE,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,             -- NULL = permanente
    revoked_at  TIMESTAMPTZ,
    revoked_by  TEXT
);
CREATE INDEX sanctions_active ON sanctions (target_id, type) WHERE revoked_at IS NULL;

CREATE TABLE device_bans (
    fingerprint TEXT PRIMARY KEY,
    sanction_id BIGINT REFERENCES sanctions(id)
);

CREATE TABLE anticheat_flags (
    id          BIGSERIAL PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id),
    detector    TEXT NOT NULL,           -- 'speed', 'apm', 'fog_probe', 'signature'...
    points      INT  NOT NULL,
    detail      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reports (                   -- denúncias de jogadores
    id          BIGSERIAL PRIMARY KEY,
    reporter_id TEXT REFERENCES accounts(id),
    target_id   TEXT REFERENCES accounts(id),
    category    TEXT NOT NULL,
    note        TEXT,
    status      TEXT NOT NULL DEFAULT 'OPEN',
    handled_by  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------- Auditoria (append-only) ----------------
CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    actor_id   TEXT NOT NULL,
    command    TEXT NOT NULL,
    target     TEXT,
    payload    JSONB,
    outcome    TEXT NOT NULL,            -- PENDING / OK / mensagem de erro
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- O usuário da aplicação só pode inserir (e atualizar o outcome de PENDING):
-- REVOKE UPDATE, DELETE ON audit_log FROM app_user; GRANT INSERT ON audit_log TO app_user;

-- ---------------- Economia (ledger de dupla entrada) ----------------
CREATE TYPE resource AS ENUM ('STEEL','FUEL','ENERGY','CREDITS');

CREATE TABLE wallets (                   -- saldo materializado; fonte da verdade = ledger
    account_id TEXT NOT NULL,            -- inclui contas de sistema 'sys:mint', 'sys:burn'
    season_id  INT,                      -- NULL para CREDITS (persistente)
    resource   resource NOT NULL,
    balance    BIGINT NOT NULL DEFAULT 0,
    UNIQUE (account_id, season_id, resource)
);

CREATE TABLE ledger_entries (
    id              BIGSERIAL PRIMARY KEY,
    idempotency_key TEXT UNIQUE,
    kind            TEXT NOT NULL,       -- DOMINATION_INCOME, SALVAGE, SPEND, SHOP_PURCHASE, ADMIN_MINT, ADMIN_DONATION...
    from_account    TEXT NOT NULL,
    to_account      TEXT NOT NULL,
    resource        resource NOT NULL,
    amount          BIGINT NOT NULL CHECK (amount > 0),
    season_id       INT,
    actor_id        TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ledger_admin ON ledger_entries (kind) WHERE kind LIKE 'ADMIN_%';

-- ---------------- Loja cosmética ----------------
-- Sem NENHUMA coluna de atributo: a garantia "zero bônus" está no schema.
CREATE TABLE cosmetics (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    slot      TEXT NOT NULL,             -- tank_camo, uniform, commander
    price     BIGINT NOT NULL,           -- em CREDITS
    asset_ref TEXT NOT NULL,
    active    BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE inventory (
    account_id  TEXT REFERENCES accounts(id),
    cosmetic_id TEXT REFERENCES cosmetics(id),
    price_paid  BIGINT NOT NULL,
    admin_free  BOOLEAN NOT NULL DEFAULT FALSE,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, cosmetic_id)
);

CREATE TABLE equipped_cosmetics (
    account_id  TEXT REFERENCES accounts(id),
    slot        TEXT NOT NULL,
    cosmetic_id TEXT REFERENCES cosmetics(id),
    PRIMARY KEY (account_id, slot)
);

-- ---------------- Mundo / temporada ----------------
CREATE TABLE seasons (
    id                   SERIAL PRIMARY KEY,
    starts_at            TIMESTAMPTZ NOT NULL,
    ends_at              TIMESTAMPTZ NOT NULL,
    server_tier_override INT CHECK (server_tier_override BETWEEN 1 AND 5)
);

CREATE TABLE sectors (
    season_id  INT REFERENCES seasons(id),
    sx         INT NOT NULL,
    sy         INT NOT NULL,
    type       TEXT NOT NULL,            -- plateau, desert, urban, port
    seed       BIGINT NOT NULL,
    owner_id   TEXT REFERENCES accounts(id),
    shard_id   TEXT,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (season_id, sx, sy)
);

CREATE TABLE domination_points (
    id          BIGSERIAL PRIMARY KEY,
    season_id   INT, sx INT, sy INT,
    is_central  BOOLEAN NOT NULL,
    x REAL NOT NULL, y REAL NOT NULL,
    owner_id    TEXT REFERENCES accounts(id),
    income_steel_per_min INT NOT NULL,
    FOREIGN KEY (season_id, sx, sy) REFERENCES sectors(season_id, sx, sy)
);

-- ---------------- Progressão ----------------
CREATE TABLE structures (
    id         BIGSERIAL PRIMARY KEY,
    season_id  INT REFERENCES seasons(id),
    owner_id   TEXT REFERENCES accounts(id),
    kind       TEXT NOT NULL,            -- HQ, REFINERY, POWER_PLANT, BARRACKS...
    level      INT  NOT NULL CHECK (level BETWEEN 1 AND 25),
    sx INT, sy INT, x REAL, y REAL,
    hp         INT  NOT NULL,
    upgrade_ends_at TIMESTAMPTZ
);

CREATE TABLE research (
    season_id  INT REFERENCES seasons(id),
    owner_id   TEXT REFERENCES accounts(id),
    tech_id    TEXT NOT NULL,
    level      INT  NOT NULL,
    PRIMARY KEY (season_id, owner_id, tech_id)
);

CREATE TABLE player_season (
    season_id       INT REFERENCES seasons(id),
    account_id      TEXT REFERENCES accounts(id),
    researched_tier INT NOT NULL DEFAULT 1 CHECK (researched_tier BETWEEN 1 AND 5),
    newbie_shield_until TIMESTAMPTZ,
    score           BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (season_id, account_id)
);

-- ---------------- Alianças ----------------
CREATE TABLE alliances (
    id         BIGSERIAL PRIMARY KEY,
    season_id  INT REFERENCES seasons(id),
    state      TEXT NOT NULL,            -- PROPOSED, ACTIVE, BROKEN, EXPIRED
    expires_at TIMESTAMPTZ,
    broken_by  TEXT REFERENCES accounts(id),
    broken_at  TIMESTAMPTZ
);

CREATE TABLE alliance_members (
    alliance_id BIGINT REFERENCES alliances(id),
    account_id  TEXT REFERENCES accounts(id),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (alliance_id, account_id)
);
