# B.A.R.D. — Bot Automated Roleplaying Database

A local-first RAG (Retrieval-Augmented Generation) assistant for tabletop RPG campaigns. B.A.R.D. ingests your campaign notes and character sheets into a vector database, then answers lore questions using a local LLM — no API costs, no internet dependency, complete privacy.

Built to integrate with [Firecast](https://firecast.app/) VTT via a custom Lua plugin.

## Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22 |
| Database | PostgreSQL + [pgvector](https://github.com/pgvector/pgvector) |
| Embeddings | [Ollama](https://ollama.com/) — `nomic-embed-text` (768-dim vectors) |
| Chat model | Ollama — `llama3.1:8b` |
| Frontend | Firecast plugin (Lua, SDK 3) |

## Setup

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Go 1.22+](https://go.dev/)
- [Ollama](https://ollama.com/)
- [Firecast](https://firecast.app/) + [SDK 3 (RDK 3.7b)](https://firecast.app/downloads/RDK3.7b.exe) — for the plugin

### 1. Pull Ollama models

```bash
ollama pull llama3.1
ollama pull nomic-embed-text
```

### 2. Start the database

```bash
docker run -d --name rag-db -p 5432:5432 \
  -e POSTGRES_PASSWORD=mysecretpassword \
  pgvector/pgvector:pg15
```

Apply the schema:

```bash
# PowerShell
Get-Content db/migrations/001_init_pgvector.sql | docker exec -i rag-db psql -U postgres

# Bash
cat db/migrations/001_init_pgvector.sql | docker exec -i rag-db psql -U postgres
```

### 3. Configure environment

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

### 4. Run the server

```bash
go run ./cmd/bot
```

The API will listen on `http://localhost:8080`.

### 5. Build and install the Firecast plugin

```bash
cd firecast-plugin
rdk p    # prepare the project (downloads SDK files into sdk/)
rdk i    # compile and install into Firecast
```

If Firecast is running, the plugin hot-reloads automatically. See [docs/architecture.md](docs/architecture.md#building-and-installing-the-plugin) for full details on `rdk` commands and distribution.

### 6. Configure and sync

In your Firecast chat, run `/lore_config` to open the plugin's settings window and set the backend address, your access token, and (optionally) B.A.R.D.'s avatar. Then run `/lore_sync` to index your campaign documents, and ask questions with `/lore <query>`.

## Recommended Hardware

An NVIDIA GPU with 8+ GB VRAM is recommended. Tested on an RTX 5060 Ti (16 GB VRAM) with 32 GB system RAM — models load entirely into VRAM, generation is near-instant, idle consumption is negligible.

## Architecture

See [docs/architecture.md](docs/architecture.md) for API endpoints, RAG pipeline details, ETL design, and deployment instructions.
