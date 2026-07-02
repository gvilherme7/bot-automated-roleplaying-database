package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateRAGResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		if payload.Model != "test-model" {
			t.Fatalf("unexpected model: %s", payload.Model)
		}
		if payload.Stream {
			t.Fatal("expected non-streaming request")
		}
		if payload.Options["temperature"] != float64(0.1) {
			t.Fatalf("unexpected temperature: %v", payload.Options["temperature"])
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(payload.Messages))
		}
		if payload.Messages[0].Role != "system" {
			t.Fatalf("expected system role, got %s", payload.Messages[0].Role)
		}
		if payload.Messages[1].Role != "user" {
			t.Fatalf("expected user role, got %s", payload.Messages[1].Role)
		}
		if !strings.Contains(payload.Messages[1].Content, "test context") {
			t.Fatalf("expected context in user message: %s", payload.Messages[1].Content)
		}
		if !strings.Contains(payload.Messages[1].Content, "test question") {
			t.Fatalf("expected question in user message: %s", payload.Messages[1].Content)
		}

		json.NewEncoder(w).Encode(OllamaChatResponse{
			Model: "test-model",
			Message: OllamaMessage{
				Role:    "assistant",
				Content: "test response",
			},
			Done: true,
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "test-model")
	resp, err := client.GenerateRAGResponse(context.Background(), "test context", "test question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "test response" {
		t.Fatalf("unexpected response: %s", resp)
	}
}

func TestGenerateRAGResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "test-model")
	_, err := client.GenerateRAGResponse(context.Background(), "test context", "test question")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGenerateEmbedding(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload OllamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload.Model != "nomic-embed-text" {
			t.Fatalf("unexpected embedding model: %s", payload.Model)
		}
		if payload.Prompt != "test text" {
			t.Fatalf("unexpected prompt: %s", payload.Prompt)
		}

		json.NewEncoder(w).Encode(OllamaEmbedResponse{
			Embedding: []float32{0.1, 0.2, 0.3},
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "test-model")
	embedding, err := client.GenerateEmbedding(context.Background(), "test text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != 3 {
		t.Fatalf("unexpected embedding length: %d", len(embedding))
	}
}

func TestGenerateEmbeddingEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(OllamaEmbedResponse{})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "test-model")
	_, err := client.GenerateEmbedding(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
