# LLM Feature: Ollama Client Integration

This document outlines the local Ollama integration used to generate Retrieval-Augmented Generation (RAG) responses and embeddings.

## Ollama Integration Details

1. **API Endpoints**: 
   - Chat: `POST /api/chat`
   - Embeddings: `POST /api/embeddings`
   - The default base URL is `http://127.0.0.1:11434`.
2. **Payload Structure**: 
   - Chat requests include `model`, `messages`, `stream: false`, and deterministic generation options.
   - Embedding requests use `model: nomic-embed-text` and send the source text as `prompt`.
3. **Role Isolation Constraints**: 
   - **System Role**: Defines B.A.R.D. as a PT-BR Game Master assistant and strict archivist.
   - **User Role**: Contains retrieved context, the user's question, and explicit rules against unsupported inference.
4. **Determinism Constraints**: 
   - **Temperature**: Hardcoded to `0.1`.
   - **Context Window**: Configured with `num_ctx: 32768` for larger lore context windows.

## Expected Runtime Characteristics

- Performance depends on the local Ollama host and available CPU/GPU resources.
- Embedding requests are concurrency-limited by the API server to reduce local model pressure.
- The app remains private by default because lore and prompts stay on the user's machine.

## Software Usage

- `NewOllamaClient(baseURL string, model string) *OllamaClient`
  - Constructs the HTTP wrapper. Defaults to `http://127.0.0.1:11434` and `llama3.1` when values are empty.
- `GenerateRAGResponse(ctx context.Context, contextText string, question string) (string, error)`
  - Generates the chat payload, executes the Ollama request, and returns the assistant message.
- `GenerateEmbedding(ctx context.Context, text string) ([]float32, error)`
  - Generates a `nomic-embed-text` embedding for semantic search.

## Test Results

Unit testing uses `net/http/httptest` to mock the local Ollama API. The tests verify chat payload structure, deterministic options, embedding requests, response parsing, and error handling without requiring a live Ollama process.

### Execution Output

```text
=== RUN   TestGenerateRAGResponse
--- PASS: TestGenerateRAGResponse (0.00s)
=== RUN   TestGenerateRAGResponseError
--- PASS: TestGenerateRAGResponseError (0.00s)
PASS
ok      rag-bot/internal/llm    0.289s
```

The assertions validate that the Go client constructs Ollama-compatible chat and embedding requests.
