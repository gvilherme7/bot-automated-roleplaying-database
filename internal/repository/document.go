package repository

import (
	"context"
	"strings"

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

// SearchSimilar returns the top `limit` chunks by cosine similarity, optionally
// scoped to documents whose metadata path contains `pathFilter`.
func (r *DocumentRepo) SearchSimilar(ctx context.Context, embedding []float32, limit int, pathFilter string) ([]Document, error) {
	var q string
	var args []any

	if pathFilter != "" {
		q = `
			SELECT id, content, embedding, metadata
			FROM campaign_documents
			WHERE metadata->>'path' ILIKE $2
			ORDER BY embedding <-> $1
			LIMIT $3
		`
		args = []any{pgvector.NewVector(embedding), "%" + pathFilter + "%", limit}
	} else {
		q = `
			SELECT id, content, embedding, metadata
			ORDER BY embedding <-> $1
			LIMIT $2
		`
		// Use subquery form to allow ORDER BY on whole table
		q = `
			SELECT id, content, embedding, metadata
			FROM campaign_documents
			ORDER BY embedding <-> $1
			LIMIT $2
		`
		args = []any{pgvector.NewVector(embedding), limit}
	}

	return r.runDocQuery(ctx, q, args...)
}

// SearchFTS returns documents matching the full-text query using PostgreSQL
// tsvector / tsquery (portuguese configuration). Useful for proper nouns and
// exact keyword matches that embeddings handle poorly.
func (r *DocumentRepo) SearchFTS(ctx context.Context, query string, limit int, pathFilter string) ([]Document, error) {
	tsQuery := buildTSQuery(query)
	if tsQuery == "" {
		return nil, nil
	}

	var q string
	var args []any

	if pathFilter != "" {
		q = `
			SELECT id, content, embedding, metadata
			FROM campaign_documents
			WHERE tsv @@ to_tsquery('portuguese', $1)
			  AND metadata->>'path' ILIKE $2
			ORDER BY ts_rank(tsv, to_tsquery('portuguese', $1)) DESC
			LIMIT $3
		`
		args = []any{tsQuery, "%" + pathFilter + "%", limit}
	} else {
		q = `
			SELECT id, content, embedding, metadata
			FROM campaign_documents
			WHERE tsv @@ to_tsquery('portuguese', $1)
			ORDER BY ts_rank(tsv, to_tsquery('portuguese', $1)) DESC
			LIMIT $2
		`
		args = []any{tsQuery, limit}
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		// FTS query can fail on unusual input; return empty rather than error
		return nil, nil
	}
	defer rows.Close()
	return scanDocs(rows)
}

// buildTSQuery converts a natural-language query into a PostgreSQL tsquery
// using OR-connected lexemes so partial matches still surface results.
func buildTSQuery(query string) string {
	words := strings.Fields(query)
	var terms []string
	for _, w := range words {
		// Strip punctuation, skip short stop words
		w = strings.Trim(w, ".,!?;:\"'()")
		if len([]rune(w)) >= 3 {
			terms = append(terms, w)
		}
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " | ")
}

func (r *DocumentRepo) runDocQuery(ctx context.Context, q string, args ...any) ([]Document, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocs(rows)
}

func scanDocs(rows pgx.Rows) ([]Document, error) {
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
	return docs, rows.Err()
}

func (r *DocumentRepo) CreateDocument(ctx context.Context, doc *Document) error {
	query := `
		INSERT INTO campaign_documents (content, embedding, metadata)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query, doc.Content, pgvector.NewVector(doc.Embedding), doc.Metadata).Scan(&doc.ID)
}

func (r *DocumentRepo) DeleteDocumentsByPath(ctx context.Context, path string) error {
	_, err := r.db.Query(ctx, `DELETE FROM campaign_documents WHERE metadata->>'path' = $1`, path)
	return err
}

func (r *DocumentRepo) GetDocumentHashByPath(ctx context.Context, path string) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx, `SELECT metadata->>'hash' FROM campaign_documents WHERE metadata->>'path' = $1 LIMIT 1`, path).Scan(&hash)
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

