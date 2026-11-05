package config

import (
	"errors"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SUPREME_ADMIN_ID", "")
	if _, err := Load(); !errors.Is(err, ErrMissing) {
		t.Fatal("deveria exigir DATABASE_URL e SUPREME_ADMIN_ID")
	}
	t.Setenv("DATABASE_URL", "postgresql://postgres.abc:senha@aws-0-sa-east-1.pooler.supabase.com:5432/postgres")
	t.Setenv("SUPREME_ADMIN_ID", "acc:gabriel")
	t.Setenv("SUPABASE_URL", "https://abc.supabase.co/")
	c, err := Load()
	if err != nil || c.Port != "8080" || c.SupabaseURL != "https://abc.supabase.co" {
		t.Fatalf("%+v %v", c, err)
	}
}
