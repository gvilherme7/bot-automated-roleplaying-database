package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"rag-bot/internal/repository"
)

type ETLJob struct {
	Path    string
	Type    string
	Title   string
	Content string
}

var (
	etlQueue = make(chan ETLJob, 100)
	etlOnce  sync.Once
)

type IngestRequest struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (s *APIServer) handleETLIngest(w http.ResponseWriter, r *http.Request) {
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

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	etlOnce.Do(func() {
		go s.etlWorker()
	})

	etlQueue <- ETLJob{
		Path:    req.Path,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

func (s *APIServer) etlWorker() {
	log.Println("ETL Worker started...")
	for job := range etlQueue {
		s.processETLJob(job)
	}
}

func (s *APIServer) processETLJob(job ETLJob) {
	ctx := context.Background()

	// Sanitize invalid bytes before hashing so dedup is stable
	job.Content = strings.ToValidUTF8(job.Content, "")

	// Hash the content
	hasher := md5.New()
	hasher.Write([]byte(job.Content))
	hashStr := hex.EncodeToString(hasher.Sum(nil))

	// Check if it already exists and hash matches
	existingHash, err := s.repo.GetDocumentHashByPath(ctx, job.Path)
	if err != nil {
		log.Printf("Error checking hash for %s: %v", job.Path, err)
		return
	}

	if existingHash == hashStr {
		log.Printf("ETL: Skipping '%s', no changes detected.", job.Title)
		return
	}

	// Hash differs or new document, so delete old chunks
	if existingHash != "" {
		log.Printf("ETL: Updating '%s', deleting old chunks...", job.Title)
		s.repo.DeleteDocumentsByPath(ctx, job.Path)
	} else {
		log.Printf("ETL: Ingesting new document '%s'...", job.Title)
	}

	processedText := sanitizeFirecastText(job.Content)

	// Transform phase
	var chunks []string
	if job.Type == "Character Sheet" {
		log.Printf("ETL: Cleaning Character Sheet '%s' with LLM...", job.Title)
		sysPrompt := "You are a data extraction assistant. Take the messy character sheet provided by the user and clean it up. " +
			"PRESERVE ALL factual information including race, class, stats, background, spells (feitiços), inventory, feats, and traits. " +
			"Structure the output using the following Portuguese section headers on their own lines, including only sections that have data:\n" +
			"## Identidade\n## Atributos\n## Feitiços\n## Inventário\n## Traços e Feitos\n## História\n\n" +
			"Rewrite everything in clear, factual Brazilian Portuguese. Do not add conversational text."
		cleanText, err := s.chatClient.GenerateRAGResponse(ctx, sysPrompt, job.Content)
		if err == nil && cleanText != "" {
			processedText = cleanText
		} else {
			log.Printf("ETL: LLM cleaning failed for '%s', using raw text. Error: %v", job.Title, err)
		}
		// Split by section headers so each section gets its own embedding
		chunks = chunkBySection(processedText, job.Title)
		if len(chunks) == 0 {
			chunks = semanticChunk(processedText, 1000, 200)
		}
	} else {
		chunks = semanticChunk(processedText, 1000, 200)
	}
	
	metadataPrefix := fmt.Sprintf("[Path: %s]\n[Type: %s]\nTitle: %s\n", job.Path, job.Type, job.Title)
	
	metadataJSON, _ := json.Marshal(map[string]string{
		"path": job.Path,
		"hash": hashStr,
	})

	for i, chunk := range chunks {
		finalChunk := metadataPrefix + chunk
		
		s.embedSem <- struct{}{}
		embedding, err := s.embedClient.GenerateEmbedding(ctx, finalChunk)
		<-s.embedSem
		
		if err != nil {
			log.Printf("ETL: Failed to generate embedding for chunk %d of '%s': %v", i, job.Title, err)
			continue
		}

		doc := &repository.Document{
			Content:   finalChunk,
			Embedding: embedding,
			Metadata:  metadataJSON,
		}

		if err := s.repo.CreateDocument(ctx, doc); err != nil {
			log.Printf("ETL: Failed to insert document chunk: %v", err)
		}
	}
	
	log.Printf("ETL: Successfully loaded '%s' into %d chunks.", job.Title, len(chunks))
}

// chunkBySection splits a character sheet by markdown ## headers, prepending the
// character name to each section so retrieval works per-topic (spells, stats, etc.)
func chunkBySection(text string, title string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var chunks []string
	var currentSection strings.Builder
	var currentHeader string

	flush := func() {
		body := strings.TrimSpace(currentSection.String())
		if body == "" {
			return
		}
		header := currentHeader
		if header == "" {
			header = "Geral"
		}
		chunk := fmt.Sprintf("Personagem: %s\nSeção: %s\n%s", title, header, body)
		chunks = append(chunks, chunk)
		currentSection.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			currentHeader = strings.TrimPrefix(line, "## ")
		} else {
			currentSection.WriteString(line)
			currentSection.WriteByte('\n')
		}
	}
	flush()
	return chunks
}

// semanticChunk splits text by newlines and periods, up to maxLen, with overlap
func semanticChunk(text string, maxLen int, overlap int) []string {
	if len(text) == 0 {
		return nil
	}

	var chunks []string
	
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	
	currentChunk := ""
	
	appendChunk := func(addition string) {
		if len(currentChunk)+len(addition) > maxLen && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			
			overlapStr := ""
			if len(currentChunk) > overlap {
				overlapStr = currentChunk[len(currentChunk)-overlap:]
			} else {
				overlapStr = currentChunk
			}
			spaceIdx := strings.Index(overlapStr, " ")
			if spaceIdx != -1 {
				overlapStr = overlapStr[spaceIdx:]
			}
			currentChunk = overlapStr + addition
		} else {
			currentChunk += addition
		}
	}

	for i, line := range lines {
		newlineStr := ""
		if i < len(lines)-1 {
			newlineStr = "\n"
		}
		
		if len(line) > maxLen {
			sentences := strings.Split(line, ". ")
			for j, sentence := range sentences {
				if j < len(sentences)-1 {
					sentence += ". "
				}
				
				if len(sentence) > maxLen {
					runes := []rune(sentence)
					for k := 0; k < len(runes); k += maxLen {
						end := k + maxLen
						if end > len(runes) {
							end = len(runes)
						}
						appendChunk(string(runes[k:end]))
					}
				} else {
					appendChunk(sentence)
				}
			}
			appendChunk(newlineStr)
		} else {
			appendChunk(line + newlineStr)
		}
	}
	
	if len(strings.TrimSpace(currentChunk)) > 0 {
		chunks = append(chunks, currentChunk)
	}
	
	return chunks
}
