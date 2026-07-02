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
    │      └─ llama3.1:8b       (chat completion)
    │
    └──► PostgreSQL + pgvector
           └─ campaign_documents table
              (content, embedding VECTOR(768), metadata JSONB)
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

1. Embed the user's question with `nomic-embed-text`
2. Search `campaign_documents` for the 30 nearest chunks by cosine distance (`<->` operator)
3. Concatenate matched chunks into a context string
4. Send context + question to `llama3.1` with a strict system prompt
5. Return the answer as JSON

### Anti-hallucination prompt design

The system prompt enforces:
- Respond only in Brazilian Portuguese (PT-BR)
- Use **only** facts explicitly stated in the retrieved context
- Never infer, deduce, or extrapolate
- Character Sheets override Session Logs on conflicting facts
- Maximum 150 words / 3 short paragraphs
- If the answer isn't in the context, reply with a fixed fallback message
- Temperature is set to `0.1` for near-deterministic output

## ETL Pipeline

### Ingestion flow (`/api/etl/ingest`)

1. **Deduplicate**: MD5-hash the content, compare against the stored hash for the same path. Skip if unchanged.
2. **Transform**: For `Character Sheet` type documents, pass the raw text through the LLM to clean and structure it in PT-BR before embedding.
3. **Chunk**: Split text at paragraph and sentence boundaries with 1000-char max chunk size and 200-char overlap.
4. **Embed**: Generate `nomic-embed-text` embeddings for each chunk (metadata prefix prepended).
5. **Store**: Insert into `campaign_documents` with JSONB metadata containing `path` and `hash`.

Jobs are queued (buffered channel, capacity 100) and processed by a single background worker. Concurrent Ollama requests are capped at 4 via semaphore.

## Firecast Plugin

The plugin (`firecast-plugin/main.lua`) provides three chat commands:

| Command | Description |
|---------|-------------|
| `/lore <question>` | Sends concurrent acknowledge + RAG requests, displays both responses in chat |
| `/lore_add <text>` | Ingests free text into the database via `/api/documents` |
| `/lore_sync` | Recursively walks the Firecast room library (character sheets, notes, session logs), extracts text via NDB XML export, and sends each item to `/api/etl/ingest` |

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

   This creates/updates `module.xml` and populates the `sdk/` folder.

2. **Compile the plugin**:

   ```bash
   rdk c
   ```

   The compiled package is written to `firecast-plugin/output/firecast-plugin.rpk`. Compilation will fail if any `.lua` file has syntax errors or `module.xml` is misconfigured.

3. **Install into Firecast**:

   ```bash
   rdk i
   ```

   This compiles and installs in one step. If there are validation errors, the installation is aborted.

#### Distributing the plugin

Share the `.rpk` file from the `output/` folder. Recipients install it by either:
- Double-clicking the `.rpk` file (Firecast must be installed)
- Opening Firecast's plugin menu, clicking "Install", and selecting the `.rpk` file

The `.rpk` file can be deleted after installation.

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
    metadata  JSONB NOT NULL DEFAULT '{}'
);

-- HNSW index for fast approximate nearest neighbor search
CREATE INDEX campaign_documents_embedding_idx
ON campaign_documents USING hnsw (embedding vector_cosine_ops);
```

## Deployment

### Local development

The standard setup (see README) uses Docker for PostgreSQL and runs the Go server directly.

### Kubernetes

Manifests are in `k8s/`:
- `deployment.yaml` — single-replica deployment with `imagePullPolicy: Never` (for local clusters like k3d/minikube)
- `secrets.yaml.example` — template for the Secret resource (copy to `secrets.yaml` and fill in base64 values)

The Dockerfile uses a multi-stage build: compile with `golang:1.25-alpine`, run on `alpine:3.19`.

Resource limits: 500m CPU / 256Mi memory. Requests: 100m CPU / 128Mi memory.
