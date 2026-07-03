package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"
)

type mockRow struct {
	doc Document
	err error
}

func (m mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	*dest[0].(*int64) = m.doc.ID
	*dest[1].(*string) = m.doc.Content
	emb := pgvector.NewVector(m.doc.Embedding)
	*dest[2].(*pgvector.Vector) = emb
	*dest[3].(*[]byte) = m.doc.Metadata
	return nil
}

type mockRows struct {
	rows []mockRow
	idx  int
	err  error
}

func (m *mockRows) Close() {}
func (m *mockRows) Err() error { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Next() bool {
	if m.idx < len(m.rows) {
		m.idx++
		return true
	}
	return false
}
func (m *mockRows) Scan(dest ...any) error {
	return m.rows[m.idx-1].Scan(dest...)
}
func (m *mockRows) Values() ([]any, error) { return nil, nil }
func (m *mockRows) RawValues() [][]byte { return nil }
func (m *mockRows) Conn() *pgx.Conn { return nil }

type mockDB struct {
	rows pgx.Rows
	err  error
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.rows, m.err
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func TestNewDocumentRepository(t *testing.T) {
	repo := NewDocumentRepository(&mockDB{})
	if repo == nil {
		t.Fatal("expected repo to be non-nil")
	}
}

func TestSearchSimilar(t *testing.T) {
	mockData := []mockRow{
		{doc: Document{ID: 1, Content: "test", Embedding: []float32{0.1, 0.2}, Metadata: []byte("{}")}},
	}
	db := &mockDB{rows: &mockRows{rows: mockData}}
	repo := NewDocumentRepository(db)

	docs, err := repo.SearchSimilar(context.Background(), []float32{0.1, 0.2}, 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].ID != 1 {
		t.Fatalf("expected document ID 1, got %d", docs[0].ID)
	}
}

func TestSearchSimilarQueryError(t *testing.T) {
	db := &mockDB{err: errors.New("query error")}
	repo := NewDocumentRepository(db)

	_, err := repo.SearchSimilar(context.Background(), []float32{0.1}, 1, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSearchSimilarScanError(t *testing.T) {
	mockData := []mockRow{
		{err: errors.New("scan error")},
	}
	db := &mockDB{rows: &mockRows{rows: mockData}}
	repo := NewDocumentRepository(db)

	_, err := repo.SearchSimilar(context.Background(), []float32{0.1}, 1, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
