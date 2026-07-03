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

	userContent := fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s\n\nRules:\n1. Respond EXCLUSIVELY in Brazilian Portuguese (PT-BR) with flawless grammar.\n2. Rely ONLY on facts explicitly stated in the Context above.\n3. NEVER infer, deduce, guess, or extrapolate. If it is not written word-for-word in the Context, do not state it.\n4. Synthesize the information naturally as if recalling from memory. NEVER reference or mention the Context, documents, chunks, archives, sources, or any internal system terminology in your answer.\n5. Keep your answer EXTREMELY CONCISE: at most 2 short paragraphs, under 120 words.\n6. If the answer is not in the Context, reply EXACTLY: \"Informação não encontrada nos arquivos da campanha.\"", contextText, question)
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

// GenerateHypotheticalAnswer produces a short hypothetical answer for HyDE retrieval.
// The result is used as the embedding input instead of the raw question, improving
// recall for questions whose wording differs from how facts are phrased in stored docs.
func (c *OllamaClient) GenerateHypotheticalAnswer(ctx context.Context, question string) (string, error) {
	sysMsg := OllamaMessage{
		Role:    "system",
		Content: "You are a concise assistant. Given the question, write a single factual sentence in Brazilian Portuguese (PT-BR) that directly answers it, as if you had access to campaign records. Use concrete, specific language. Maximum 40 words. If you don't know, make a reasonable guess — the goal is embedding similarity, not accuracy.",
	}
	usrMsg := OllamaMessage{
		Role:    "user",
		Content: question,
	}
	payload := OllamaChatRequest{
		Model:    c.model,
		Messages: []OllamaMessage{sysMsg, usrMsg},
		Stream:   false,
		Options: map[string]any{
			"temperature": 0.0,
			"num_ctx":     512,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var res OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Message.Content, nil
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
