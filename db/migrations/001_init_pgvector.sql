CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS campaign_documents (
    id BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL,
    embedding VECTOR(768),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS campaign_documents_embedding_idx 
ON campaign_documents USING hnsw (embedding vector_cosine_ops);
