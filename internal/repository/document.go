package repository

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type Document struct {
	ID        int64
	Content   string
	Embedding []float32
	Metadata  []byte
}

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type DocumentRepo struct {
	db DB
}

func NewDocumentRepository(db DB) *DocumentRepo {
	return &DocumentRepo{db: db}
}

func (r *DocumentRepo) SearchSimilar(ctx context.Context, embedding []float32, limit int) ([]Document, error) {
	query := `
		SELECT id, content, embedding, metadata
		FROM campaign_documents
		ORDER BY embedding <-> $1
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		var emb pgvector.Vector
		if err := rows.Scan(&doc.ID, &doc.Content, &emb, &doc.Metadata); err != nil {
			return nil, err
		}
		doc.Embedding = emb.Slice()
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *DocumentRepo) CreateDocument(ctx context.Context, doc *Document) error {
	query := `
		INSERT INTO campaign_documents (content, embedding, metadata)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, doc.Content, pgvector.NewVector(doc.Embedding), doc.Metadata).Scan(&doc.ID)
	return err
}

func (r *DocumentRepo) DeleteDocumentsByPath(ctx context.Context, path string) error {
	query := `DELETE FROM campaign_documents WHERE metadata->>'path' = $1`
	_, err := r.db.Query(ctx, query, path)
	return err
}

func (r *DocumentRepo) GetDocumentHashByPath(ctx context.Context, path string) (string, error) {
	query := `SELECT metadata->>'hash' FROM campaign_documents WHERE metadata->>'path' = $1 LIMIT 1`
	var hash string
	err := r.db.QueryRow(ctx, query, path).Scan(&hash)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

func (r *DocumentRepo) Close() {
	if p, ok := r.db.(*pgxpool.Pool); ok {
		p.Close()
	}
}
