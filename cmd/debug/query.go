package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"rag-bot/internal/config"
	"rag-bot/internal/llm"
	"rag-bot/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := repository.NewDocumentRepository(pool)
	client := llm.NewOllamaClient(cfg.OllamaURL, cfg.LLMModel)

	query := "qual a raça do daelirn"
	emb, err := client.GenerateEmbedding(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}

	docs, err := repo.SearchSimilar(context.Background(), emb, 3)
	if err != nil {
		log.Fatal(err)
	}

	for i, d := range docs {
		fmt.Printf("--- Chunk %d ---\n%s\n\n", i+1, d.Content)
	}
}
