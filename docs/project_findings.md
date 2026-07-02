# Project Findings

## Summary

This repository contains a Go backend and a Firecast plugin for B.A.R.D., a tabletop RPG Game Master assistant backed by Retrieval-Augmented Generation (RAG).

The backend stores campaign documents in PostgreSQL with `pgvector`, generates semantic embeddings with local Ollama, retrieves the most relevant lore chunks, and produces concise Brazilian Portuguese answers through a local chat model.

## Current Architecture

- Backend language: Go.
- Main entrypoint: `cmd/bot/main.go`.
- HTTP API package: `internal/api`.
- Configuration package: `internal/config`.
- LLM/embedding client: `internal/llm/ollama_client.go`.
- Document repository: `internal/repository/document.go`.
- Database migration: `db/migrations/001_init_pgvector.sql`.
- Firecast plugin source: `firecast-plugin/main.lua` and `firecast-plugin/module.xml`.
- Deployment files: `Dockerfile` and `k8s/`.

## Runtime Flow

1. Firecast sends lore or ingestion requests to the Go API with `Authorization: Bearer <PLUGIN_API_KEY>`.
2. The API validates the request.
3. For queries, the API embeds the question with Ollama's `nomic-embed-text` model.
4. PostgreSQL retrieves similar chunks using `pgvector` cosine distance.
5. The retrieved context and question are sent to the local chat model.
6. The model answers in PT-BR under strict anti-hallucination instructions.

## API Surface

- `POST /api/lore`: answers a lore question using RAG.
- `POST /api/acknowledge`: returns a short Portuguese acknowledgment before a longer lookup.
- `POST /api/documents`: legacy document ingestion endpoint.
- `POST /api/etl/ingest`: queues structured ingestion jobs and chunks content for embedding.

## Important Findings

- The current implementation is Ollama-based and local-first.
- `docs/llm_feature.md` still describes Groq integration, which does not match the active implementation.
- `README.md` is closer to the current code because it describes Ollama and local inference.
- `docs/database_feature.md` mentions 1536-dimensional embeddings, but the migration uses `VECTOR(768)`, matching `nomic-embed-text`.
- `k8s/secrets.yaml` contained a real-looking base64-encoded API key and should not be used for live credentials in source control.
- `.env` contains local credentials and should remain untracked.
- `bot.exe`, `.rpk`, `.zip`, `firecast-plugin/output/`, and `Biblioteca.html` appear to be generated or local artifacts and should remain untracked.

## Recommended Next Steps

- Update `docs/llm_feature.md` to describe Ollama instead of Groq.
- Add integration tests around the HTTP handlers with mocked repository and LLM clients.
- Consider replacing MD5 content hashing in ETL with SHA-256 for stronger collision resistance.
- Add graceful shutdown for the HTTP server and database pool.
- Add a Kubernetes `Service` manifest if the deployment is intended to be usable directly from a cluster.
