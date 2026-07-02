# Game Master Bot (Local RAG)

A fully autonomous, 100% local Discord and Firecast Game Master bot. This application leverages a hybrid Retrieval-Augmented Generation (RAG) architecture powered by Ollama to instantly analyze your tabletop roleplaying notes and character sheets, and act as a conversational GM assistant.

## Features
- **100% Local Inference**: Zero API costs, zero internet dependency, and complete privacy. Your lore never leaves your computer.
- **RAG Architecture**: Instantly searches through hundreds of your campaign notes and character sheets to answer specific lore questions.
- **Firecast Integration**: Automatically syncs campaign documents from your Firecast RPG table into a high-speed vector database.

## Architecture & Software Stack
1. **Language**: Go (Golang) 1.26
2. **Database**: PostgreSQL (Dockerized)
   - **Extension**: `pgvector` (Stores high-dimensional AI vectors for fast semantic searches).
3. **Local AI Engine**: [Ollama](https://ollama.com/)
   - **Generation Model**: `llama3.1:8b` (Meta's Llama 3.1 8-billion parameter model for reasoning).
   - **Embedding Model**: `nomic-embed-text` (Generates 768-dimensional semantic embeddings for your campaign lore).

## Recommended Hardware
This system is highly optimized for modern consumer hardware. 
- **GPU Setup**: An NVIDIA GPU with at least 8GB of VRAM is recommended. 
- **Tested Performance**: On an **NVIDIA RTX 5060 Ti (16GB VRAM) paired with 32GB system RAM**, the models load entirely into GPU VRAM. 
  - Token generation is virtually instantaneous. 
  - The system will safely consume ~5-6GB of VRAM while actively generating answers.
  - Idle resource consumption is practically zero.

## Setup Instructions

### 1. Install Dependencies
You will need the following software installed on your machine:
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for the vector database)
- [Go](https://go.dev/) (to compile and run the backend server)
- [Ollama](https://ollama.com/) (to run the AI models locally)

### 2. Configure Ollama Models
Open your terminal and pull the necessary models onto your local machine:
```bash
ollama pull llama3.1
ollama pull nomic-embed-text
```
*Note: Ensure the Ollama background service is running before proceeding.*

### 3. Spin Up the Database
Start the PostgreSQL container equipped with `pgvector` by running:
```bash
docker run -d --name rag-db -p 5432:5432 -e POSTGRES_PASSWORD=mysecretpassword pgvector/pgvector:pg15
```

Once the container is running, initialize the schema:
```bash
Get-Content db/migrations/001_init_pgvector.sql | docker exec -i rag-db psql -U postgres
```

### 4. Configure Environment Variables
Create a file named `.env` in the root directory of this project and add the following database connection string:
```env
DATABASE_URL=postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable
PLUGIN_API_KEY=change_me
```

### 5. Launch the Bot
Run the Go backend server:
```bash
go run ./cmd/bot
```
The server will boot up and listen on `http://localhost:8080`.

### 6. Synchronize Lore in Firecast
In your Firecast VTT chat, run the synchronization command:
```text
/lore_sync
```
The bot will recursively scan your character sheets and notes, generate `nomic-embed-text` vector embeddings, and securely insert them into your local Postgres database.

### 7. Ask the GM
You can now ask the Game Master questions regarding anything in your notes!
```text
/lore Who is the most dangerous character in the campaign?
```
The Go server will locate the most contextually relevant documents in the database and stream them to your local `llama3.1` model to draft a lore-accurate response.
