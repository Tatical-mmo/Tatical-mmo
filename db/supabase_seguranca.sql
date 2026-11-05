-- Rode no Supabase → SQL Editor DEPOIS do schema.sql.
--
-- Por quê: o Supabase publica toda tabela do schema "public" na API REST, acessível
-- com a chave "anon" (que é pública). Sem RLS, qualquer pessoa poderia ler ou alterar
-- carteiras, sanções e o audit_log — inclusive se dar recursos infinitos.
--
-- Ligando RLS sem nenhuma policy, a API REST fica bloqueada para anon/authenticated.
-- O servidor do jogo conecta direto no Postgres (DATABASE_URL) e não é afetado.

DO $$
DECLARE t record;
BEGIN
  FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
    EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', t.tablename);
    EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', t.tablename);
    EXECUTE format('REVOKE ALL ON public.%I FROM anon, authenticated', t.tablename);
  END LOOP;
END $$;

-- Sequências (BIGSERIAL) também não devem ser usadas pela API.
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated;

-- Conferência: todas as linhas devem mostrar rowsecurity = true
SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname = 'public' ORDER BY 1;
