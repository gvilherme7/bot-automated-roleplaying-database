package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type OllamaClient struct {
	client  *http.Client
	baseURL string
	model   string
}

func NewOllamaClient(baseURL string, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = "llama3.1"
	}
	return &OllamaClient{
		client:  &http.Client{},
		baseURL: baseURL,
		model:   model,
	}
}

// Structs for Chat
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type OllamaChatResponse struct {
	Model   string        `json:"model"`
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// Structs for Embeddings
type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Implement ChatClient Interface
func (c *OllamaClient) GenerateRAGResponse(ctx context.Context, contextText string, question string) (string, error) {
	sysMsg := OllamaMessage{
		Role:    "system",
		Content: "You are B.A.R.D., an eloquent, poetic, and storytelling human-like Bard who serves as the Game Master assistant. While your tone is charming and musical, you are also a STRICT ARCHIVIST. You MUST NEVER invent facts, extrapolate, or infer anything that is not literally written in the context. You MUST communicate entirely in Brazilian Portuguese (PT-BR) with flawless grammar and spelling.",
	}

	userContent := fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s\n\nRules:\n1. Respond EXCLUSIVELY in Brazilian Portuguese (PT-BR) with flawless grammar, avoiding misspellings like 'Néutro'.\n2. Rely ONLY on the clear facts explicitly stated in the Context.\n3. UNDER NO CIRCUMSTANCES should you infer, deduce, guess, or extrapolate. NÃO use expressões como 'podemos inferir'. Se a informação não estiver escrita textualmente, não a invente.\n4. Pay close attention to the '[Title: ...]' and '[Type: ...]' of each document.\n5. [Type: Character Sheet] documents are the ABSOLUTE source of truth for biological facts, classes, and stats. They supersede [Type: Session Log] documents in conflicts.\n6. Synthesize the information naturally. DO NOT cite 'Document Chunks', 'Chunks', 'Contexto', '[Type: Character Sheet]', '[Type: Session Log]', '[Title]', or use the word 'documento'. State the facts as if recalling them from memory.\n7. Keep your answer EXTREMELY CONCISE. Never write more than 3 short paragraphs. Limit your answer to 150 words.\n8. If the answer cannot be directly found and validated by reading the Context, you MUST abort and reply EXACTLY with: \"Informação não encontrada nos arquivos da campanha.\"", contextText, question)
	usrMsg := OllamaMessage{
		Role:    "user",
		Content: userContent,
	}

	payload := OllamaChatRequest{
		Model:    c.model,
		Messages: []OllamaMessage{sysMsg, usrMsg},
		Stream:   false,
		Options: map[string]any{
			"temperature": 0.1,
			"num_ctx":     32768,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var resPayload OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&resPayload); err != nil {
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}

	return resPayload.Message.Content, nil
}

// Implement EmbeddingClient Interface
func (c *OllamaClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload := OllamaEmbedRequest{
		Model:  "nomic-embed-text",
		Prompt: text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	url := c.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var resPayload OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&resPayload); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	if len(resPayload.Embedding) == 0 {
		return nil, errors.New("no embeddings returned from Ollama API")
	}

	return resPayload.Embedding, nil
}
