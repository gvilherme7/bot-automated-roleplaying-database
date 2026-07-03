# Architecture

## System Overview

```
Firecast VTT (Lua plugin)
    │
    │  HTTP + Bearer auth
    ▼
Go API Server (:8080)
    │
    ├──► Ollama (local)
    │      ├─ nomic-embed-text  (embeddings, 768-dim)
    │      └─ qwen2.5:14b       (chat completion)
    │
    └──► PostgreSQL + pgvector
           └─ campaign_documents table
              (content, embedding VECTOR(768), metadata JSONB, tsv tsvector)
```

## API Endpoints

All endpoints require `Authorization: Bearer <PLUGIN_API_KEY>` and accept `POST` with JSON body.

| Endpoint | Purpose | Request body |
|----------|---------|-------------|
| `/api/lore` | Answer a lore question via RAG | `{"query": "..."}` |
| `/api/acknowledge` | Quick acknowledgment while RAG processes | `{"query": "..."}` |
| `/api/documents` | Legacy sync ingestion | `{"query": "..."}` (content as query) |
| `/api/etl/ingest` | Structured ETL ingestion with dedup | `{"path": "...", "type": "...", "title": "...", "content": "..."}` |

## RAG Pipeline

### Query flow (`/api/lore`)

Three retrieval strategies run in parallel and are merged before selection:

1. **Vector search** — embed the user's question with `nomic-embed-text`, find the 30 nearest chunks by cosine distance
2. **HyDE (Hypothetical Document Embeddings)** — ask the LLM to generate a short hypothetical answer (PT-BR, max 40 words, temp 0.0), embed that, find the 20 nearest chunks. This improves recall when the question wording differs from how facts are stored.
3. **Full-text search (FTS)** — run a `to_tsquery('portuguese', ...)` query against the `tsv` tsvector column, ranked by `ts_rank`. Returns up to 20 results. This is particularly effective for proper nouns (character names, place names) that don't embed well.

Results are merged in priority order (vector → HyDE → FTS), deduplicated by ID, then filtered to at most 2 chunks per source path, capping at 10 total diverse chunks for the LLM context.

If the query contains an explicit group or arc reference (`"grupo 3"`, `"arco 2"`), all three searches are scoped to documents whose `metadata->>'path'` matches that string, preventing irrelevant groups from flooding the context.

### Anti-hallucination prompt design

The system prompt enforces:
- Respond only in Brazilian Portuguese (PT-BR)
- Use **only** facts explicitly stated in the retrieved context
- Never infer, deduce, or extrapolate
- Maximum 120 words / 2 short paragraphs
- If the answer isn't in the context, reply with a fixed fallback message
- Temperature is set to `0.1` for near-deterministic output

## ETL Pipeline

### Ingestion flow (`/api/etl/ingest`)

1. **Sanitize**: Strip invalid UTF-8 bytes (`strings.ToValidUTF8`), then remove Firecast formatting artifacts: color codes (`$FFAABBCC`), font declarations (`Roboto txt`), encoding headers, session schedule boilerplate, waitlist paragraphs, and Discord links.
2. **Deduplicate**: MD5-hash the sanitized content, compare against the stored hash for the same path. Skip if unchanged; delete old chunks if changed.
3. **Transform**:
   - *Character Sheets*: pass through the LLM to clean and restructure into labeled PT-BR sections (`## Identidade`, `## Atributos`, `## Feitiços`, `## Inventário`, `## Traços e Feitos`, `## História`).
   - *Session Logs and other types*: no LLM transform.
4. **Chunk**:
   - *Character Sheets*: split by `##` section headers so each section gets its own embedding. Each chunk is prefixed with `Personagem: <name>\nSeção: <section>` for targeted retrieval.
   - *All other types*: semantic chunking at paragraph/sentence boundaries (1000-char max, 200-char overlap).
5. **Embed**: Generate `nomic-embed-text` embeddings for each chunk (metadata prefix prepended).
6. **Store**: Insert into `campaign_documents` with JSONB metadata containing `path` and `hash`. The `tsv` column is populated automatically via a generated column.

Jobs are queued (buffered channel, capacity 100) and processed by a single background worker. Concurrent Ollama requests are capped at 4 via semaphore.

## Firecast Plugin

The plugin (`firecast-plugin/main.lua`) provides three chat commands:

| Command | Description |
|---------|-------------|
| `/lore <question>` | Sends concurrent acknowledge + RAG requests, displays both responses in chat |
| `/lore_add <text>` | Ingests free text into the database via `/api/documents` |
| `/lore_sync` | Recursively walks the Firecast room library (character sheets, notes, session logs), extracts text via NDB XML export, and sends each item to `/api/etl/ingest` |

All HTTP requests include timeouts to prevent Firecast from hanging when the backend is unreachable:
- ETL ingest: 5s (fast-fail, queue continues)
- `/lore` query: 90s (allows LLM generation time)
- `/lore` acknowledge: 10s

The plugin impersonates a character named "B.A.R.D" in chat responses. Messages are consumed so slash commands don't appear to other users.

The SDK files (`firecast-plugin/sdk/`) are a third-party dependency and are not tracked in git.

### Building and installing the plugin

#### Prerequisites

- [Firecast](https://firecast.app/) installed
- [Firecast SDK 3 (RDK 3.7b)](https://firecast.app/downloads/RDK3.7b.exe) installed — this adds the `rdk` CLI tool
- A text editor (e.g. VS Code, Notepad++)

The full SDK documentation is available at https://firecast.app/sdk3/RRPG%20SDK%203.html

#### rdk commands

| Command | Purpose |
|---------|---------|
| `rdk p` | **Prepare** — initializes or updates a directory as a plugin project. Creates `module.xml` and copies the SDK files into `sdk/`. Run this after downloading a new SDK version to update each project. |
| `rdk c` | **Compile** — validates all `.lua` and `.lfm` files, then packages everything into `output/<module-name>.rpk`. Files and directories prefixed with `__` are excluded. |
| `rdk i` | **Install** — compiles and installs the plugin into Firecast. If Firecast is running, the old version is unloaded and the new one is hot-reloaded. If Firecast is closed, the plugin installs offline and loads on next launch. |

#### Step-by-step

1. **Prepare the project** (first time or after SDK update):

   ```bash
   cd firecast-plugin
   rdk p
   ```

2. **Compile the plugin**:

   ```bash
   rdk c
   ```

3. **Install into Firecast**:

   ```bash
   rdk i
   ```

#### Plugin project structure

```
firecast-plugin/
├── module.xml          # Plugin manifest (id, version, metadata)
├── main.lua            # Entry point — chat command handlers, HTTP calls, NDB sync
├── sdk/                # Firecast SDK 3 (auto-populated by `rdk p`, not tracked in git)
└── output/             # Build output (not tracked in git)
    └── firecast-plugin.rpk
```

## Database Schema

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE campaign_documents (
    id        BIGSERIAL PRIMARY KEY,
    content   TEXT NOT NULL,
    embedding VECTOR(768),
    metadata  JSONB NOT NULL DEFAULT '{}',
    tsv       tsvector GENERATED ALWAYS AS (to_tsvector('portuguese', content)) STORED
);

-- HNSW index for fast approximate nearest neighbor search
CREATE INDEX campaign_documents_embedding_idx
ON campaign_documents USING hnsw (embedding vector_cosine_ops);

-- GIN index for full-text search
CREATE INDEX campaign_documents_tsv_idx
ON campaign_documents USING GIN(tsv);
```

## Deployment

### Local development

The standard setup (see README) uses Docker for PostgreSQL and runs the Go server directly.

```bash
# Start PostgreSQL
docker run -d --name rag-db -e POSTGRES_PASSWORD=mysecretpassword -p 5432:5432 ankane/pgvector

# Apply migrations
docker exec rag-db psql -U postgres -f db/migrations/001_init_pgvector.sql
docker exec rag-db psql -U postgres -f db/migrations/002_add_fts.sql

# Start Go server
go run ./cmd/bot
```

### Kubernetes

Manifests are in `k8s/`:
- `deployment.yaml` — single-replica deployment with `imagePullPolicy: Never` (for local clusters like k3d/minikube)
- `secrets.yaml.example` — template for the Secret resource (copy to `secrets.yaml` and fill in base64 values)

The Dockerfile uses a multi-stage build: compile with `golang:1.25-alpine`, run on `alpine:3.19`.

Resource limits: 500m CPU / 256Mi memory. Requests: 100m CPU / 128Mi memory.
