package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"rag-bot/internal/repository"
)

type DocumentRepository interface {
	SearchSimilar(ctx context.Context, embedding []float32, limit int) ([]repository.Document, error)
	CreateDocument(ctx context.Context, doc *repository.Document) error
	DeleteDocumentsByPath(ctx context.Context, path string) error
	GetDocumentHashByPath(ctx context.Context, path string) (string, error)
}

type ChatClient interface {
	GenerateRAGResponse(ctx context.Context, contextText string, question string) (string, error)
}

type EmbeddingClient interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type APIServer struct {
	repo       DocumentRepository
	chatClient ChatClient
	embedClient EmbeddingClient
	pluginKey  string
	listenAddr string
	embedSem   chan struct{}
}

func NewAPIServer(repo DocumentRepository, chatClient ChatClient, embedClient EmbeddingClient, pluginKey string, listenAddr string) *APIServer {
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	return &APIServer{
		repo:       repo,
		chatClient: chatClient,
		embedClient: embedClient,
		pluginKey:  pluginKey,
		listenAddr: listenAddr,
		embedSem:   make(chan struct{}, 4), // Limit to 4 concurrent Ollama requests
	}
}

type LoreRequest struct {
	Query string `json:"query"`
}

type LoreResponse struct {
	Answer string `json:"answer"`
	Error  string `json:"error,omitempty"`
}

func (s *APIServer) handleLore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != s.pluginKey {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req LoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Constraint: Length limits (Prompt Injection defense against buffer stuffing)
	if len(req.Query) > 500 {
		http.Error(w, "Query exceeds maximum length of 500 characters", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	s.embedSem <- struct{}{}
	embedding, err := s.embedClient.GenerateEmbedding(ctx, query)
	<-s.embedSem
	
	if err != nil {
		log.Printf("Failed to generate embedding: %v", err)
		http.Error(w, "LLM Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	docs, err := s.repo.SearchSimilar(ctx, embedding, 30)
	if err != nil {
		log.Printf("Failed to search database: %v", err)
		http.Error(w, "Database Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var contextBuilder strings.Builder
	log.Printf("--- Retrieved Chunks for Query: '%s' ---", query)
	for i, doc := range docs {
		preview := doc.Content
		if len(preview) > 150 {
			preview = preview[:150]
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		log.Printf("Chunk %d: %s...", i+1, preview)

		contextBuilder.WriteString(doc.Content)
		contextBuilder.WriteString("\n\n")
	}
	log.Printf("-----------------------------------------")

	answer, err := s.chatClient.GenerateRAGResponse(ctx, contextBuilder.String(), query)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		http.Error(w, "LLM Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res := LoreResponse{Answer: answer}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(res)
}

func (s *APIServer) handleAcknowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != s.pluginKey {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req LoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	systemPrompt := "You are B.A.R.D., an objective RPG Game Master assistant. The user just asked a question."
	userPrompt := "User Question: '" + query + "'\n\nProvide a very brief, 1-sentence acknowledgment (max 15 words) in Portuguese that you are searching the archives for that information. Do not answer the question yet, just acknowledge you are looking."
	
	answer, err := s.chatClient.GenerateRAGResponse(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("Failed to generate acknowledgment: %v", err)
		http.Error(w, "LLM Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res := LoreResponse{Answer: answer}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(res)
}

func (s *APIServer) handleAddLore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != s.pluginKey {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req LoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(req.Query)
	if content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	var logTitle string
	if strings.HasPrefix(content, "[Path: ") {
		endIdx := strings.Index(content, "]")
		if endIdx != -1 {
			logTitle = content[7:endIdx]
		} else {
			logTitle = "Unknown Document"
		}
	} else if strings.HasPrefix(content, "Title: ") {
		parts := strings.SplitN(content, "\n", 2)
		logTitle = strings.TrimPrefix(parts[0], "Title: ")
	} else {
		logTitle = "Unknown Document"
	}

	// Extract the metadata block to prepend to EVERY chunk
	var metadataPrefix string
	var body string
	
	if strings.HasPrefix(content, "[Path: ") {
		idx := strings.Index(content, "Title: ")
		if idx != -1 {
			newlineIdx := strings.Index(content[idx:], "\n")
			if newlineIdx != -1 {
				metadataPrefix = content[:idx+newlineIdx+1]
				body = content[idx+newlineIdx+1:]
			} else {
				metadataPrefix = content
				body = ""
			}
		} else {
			body = content
		}
	} else if strings.HasPrefix(content, "Title: ") {
		parts := strings.SplitN(content, "\n", 2)
		if len(parts) == 2 {
			metadataPrefix = parts[0] + "\n"
			body = parts[1]
		} else {
			body = content
		}
	} else {
		body = content
	}

	// Use context.Background() instead of r.Context() because Firecast might time out
	// and drop the HTTP connection before Ollama finishes generating the embedding.
	// We want the ingestion to complete regardless of the client disconnecting.
	ctx := context.Background()
	
	chunks := chunkString(body, 4000)
	
	log.Printf("Ingesting '%s': %d chunks to process...", logTitle, len(chunks))
	
	for i, chunk := range chunks {
		log.Printf("Ingesting '%s': generating embedding for chunk %d/%d...", logTitle, i+1, len(chunks))
		
		finalChunk := metadataPrefix + chunk
		
		s.embedSem <- struct{}{}
		embedding, err := s.embedClient.GenerateEmbedding(ctx, finalChunk)
		<-s.embedSem
		
		if err != nil {
			log.Printf("Failed to generate embedding for chunk: %v", err)
			continue
		}

		doc := &repository.Document{
			Content:   finalChunk,
			Embedding: embedding,
			Metadata:  []byte("{}"),
		}

		if err := s.repo.CreateDocument(ctx, doc); err != nil {
			log.Printf("Failed to insert document chunk: %v", err)
			continue
		}
	}

	log.Printf("Successfully ingested '%s' (split into %d chunks, original length: %d chars) into RAG database.", logTitle, len(chunks), len(content))
	
	res := LoreResponse{Answer: "Lore added successfully!"}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(res)
}

// Helper to split long strings into smaller chunks for the embedding model
func chunkString(s string, chunkSize int) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func (s *APIServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/lore", s.handleLore)
	mux.HandleFunc("/api/acknowledge", s.handleAcknowledge)
	mux.HandleFunc("/api/documents", s.handleAddLore) // Legacy sync endpoint
	mux.HandleFunc("/api/etl/ingest", s.handleETLIngest)
	log.Printf("API Server listening on %s", s.listenAddr)
	return http.ListenAndServe(s.listenAddr, mux)
}
