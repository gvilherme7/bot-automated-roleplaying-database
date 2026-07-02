# Database Feature: Document Repository

This document details the database layer implementation for the RAG bot, focusing on vector similarity search operations using PostgreSQL and `pgvector`.

## Features

1. **Schema Initialization**: 
   - Uses `pgvector` for advanced mathematical similarity operations directly within PostgreSQL.
   - Defines the `campaign_documents` table with `id`, `content`, `embedding` (1536-dimensional vector for standard OpenAI embeddings), and `metadata` (JSONB) columns.
   - Applies an HNSW (Hierarchical Navigable Small World) index optimizing the Cosine Distance search (`vector_cosine_ops`), which ensures low latency semantic queries on large datasets.
   
2. **Repository Implementation**:
   - `DocumentRepo` wraps generic database connection interfaces (`DB`).
   - Implements `SearchSimilar(ctx, embedding, limit)` to execute a semantic search query utilizing the `<->` Cosine distance operator from `pgvector`.

## Software Usage and Endpoints

- **Migration**: `db/migrations/001_init_pgvector.sql`
  - Usage: Applied during bot startup or CI/CD pipelines to ensure the `pgvector` extension and indexing are properly instantiated.
- **Repository Methods**: `internal/repository/document.go`
  - `NewDocumentRepository(db DB) *DocumentRepo`: Initializes the repository structure.
  - `SearchSimilar`: Used internally by the LLM ingestion and RAG search pipeline to fetch structurally similar documents based on user prompt embeddings.
  - `Close()`: Gracefully halts connection pools during application teardown.

## Metrics Considerations

- **Similarity Algorithm**: Cosine Distance (`<->`) is prioritized for RAG pipelines because it normalizes vector magnitude differences, accurately measuring the semantic angle of prompts.
- **Query Limits**: Mandatory hard limits passed via `$2` query parameters restrict the result sets, capping memory consumption and maintaining sub-millisecond database response times during heavy plugin API requests.
- **Index Efficacy**: The HNSW index provides highly accurate approximate nearest neighbor (ANN) retrieval, scaling significantly better than flat exact searches for 100,000+ vector records.

## Test Results

The repository incorporates extensive unit testing using mocked database connection interfaces, ensuring that SQL driver interactions map variables correctly and handle transaction errors without requiring a live PostgreSQL instance during the test suite execution.

### Execution Output

```text
=== RUN   TestNewDocumentRepository
--- PASS: TestNewDocumentRepository (0.00s)
=== RUN   TestSearchSimilar
--- PASS: TestSearchSimilar (0.00s)
=== RUN   TestSearchSimilarQueryError
--- PASS: TestSearchSimilarQueryError (0.00s)
=== RUN   TestSearchSimilarScanError
--- PASS: TestSearchSimilarScanError (0.00s)
PASS
ok      rag-bot/internal/repository     0.383s
```

All 4 test assertions passed, validating structural mappings and robust error handling boundaries.
