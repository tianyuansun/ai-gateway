package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/ingress"
	"github.com/tianyuansun/ai-gateway/pkg/logging"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "gateway.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	gw := ingress.NewGateway(cfg)
	if err := gw.Start(); err != nil {
		log.Fatalf("failed to start gateway: %v", err)
	}

	mux := http.NewServeMux()

	modelsHandler := ingress.NewModelsHandler(cfg, gw.HealthChecker())
	codexConfigHandler := ingress.NewCodexConfigHandler(cfg)

	mux.HandleFunc("/v1/responses", gw.ServeResponses)
	mux.HandleFunc("/v1/chat/completions", gw.ServeChat)
	mux.HandleFunc("/v1/messages", gw.ServeMessages)
	mux.Handle("/v1/models", modelsHandler)
	mux.Handle("/v1/codex-config", codexConfigHandler)

	healthHandler := ingress.NewHealthHandler(cfg, gw.HealthChecker())
	mux.Handle("/health", healthHandler)

	mux.Handle("/admin/log-level", logging.AdminHandler())
	mux.HandleFunc("/v1/responses/compact", gw.ServeCompact)

	server := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: mux,
	}

	go func() {
		log.Printf("AI Gateway %s (%s) built %s", version, commit, date)
		log.Printf("listening on %s", cfg.Server.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	server.Shutdown(context.Background())
}
