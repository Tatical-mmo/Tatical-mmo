// Servidor do jogo. Por enquanto expõe só endpoints de saúde e status;
// o loop de simulação e o WebSocket entram nas próximas fases.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/seu-usuario/tactical-mmo/internal/config"
)

var startedAt = time.Now()

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Não derruba o servidor: sobe em modo limitado e avisa no /status.
		log.Printf("AVISO: %v — servidor em modo sem banco", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"jogo":            "Tatical-mmo",
			"versao":          "0.1.0",
			"online_ha":       time.Since(startedAt).Round(time.Second).String(),
			"banco_config":    cfg.DatabaseURL != "",
			"admin_config":    cfg.SupremeAdminID != "",
			"supabase_config": cfg.SupabaseURL != "",
		})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Tatical-mmo</title>
<body style="font-family:sans-serif;background:#111;color:#eee;padding:2rem">
<h1>Tatical-mmo: servidor online</h1><p>Veja <a style="color:#8af" href="/status">/status</a>.</p></body>`))
	})

	port := cfg.Port
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("Servidor ouvindo na porta %s", port)
	log.Fatal(srv.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
