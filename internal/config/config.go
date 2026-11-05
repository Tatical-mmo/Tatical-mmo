// Package config lê a configuração do servidor a partir de variáveis de ambiente.
// Nunca coloque senhas ou chaves no código: o repositório é público.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	// Conexão direta com o Postgres do Supabase (Settings → Database → Connection string).
	// O servidor do jogo usa ESTA conexão, não a API REST do Supabase.
	DatabaseURL string
	// URL do projeto Supabase (https://<ref>.supabase.co) — opcional, só para Auth/Storage.
	SupabaseURL string
	// Chave service_role — opcional, SÓ no servidor, nunca no cliente do jogo.
	SupabaseServiceKey string
	// ID da sua conta: o Admin Supremo.
	SupremeAdminID string
	// Porta HTTP/WebSocket do Gateway.
	Port string
}

var ErrMissing = errors.New("config: variável obrigatória ausente")

func Load() (Config, error) {
	c := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		SupabaseURL:        strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseServiceKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupremeAdminID:     os.Getenv("SUPREME_ADMIN_ID"),
		Port:               os.Getenv("PORT"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.SupremeAdminID == "" {
		missing = append(missing, "SUPREME_ADMIN_ID")
	}
	if len(missing) > 0 {
		return c, errors.Join(ErrMissing, errors.New(strings.Join(missing, ", ")))
	}
	return c, nil
}
