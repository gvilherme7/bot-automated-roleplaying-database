package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"rag-bot/internal/llm"
	"rag-bot/internal/repository"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := repository.NewDocumentRepository(pool)
	client := llm.NewOllamaClient("http://127.0.0.1:11434", "llama3.1")

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
