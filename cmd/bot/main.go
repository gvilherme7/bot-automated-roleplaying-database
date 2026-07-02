package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"rag-bot/internal/api"
	"rag-bot/internal/config"
	"rag-bot/internal/llm"
	"rag-bot/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: failed to initialize database connection: %v", err)
	}
	repo := repository.NewDocumentRepository(pool)

	ollamaClient := llm.NewOllamaClient(cfg.OllamaURL, cfg.LLMModel)

	apiServer := api.NewAPIServer(repo, ollamaClient, ollamaClient, cfg.PluginAPIKey, ":8080")
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server stopped: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Println("Shutting down gracefully...")
	return nil
}
