package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL  string
	PluginAPIKey string
	OllamaURL    string
	LLMModel     string
}

func Load() (*Config, error) {
	if err := godotenv.Overload(); err != nil {
		log.Printf("Warning: error loading .env file: %v", err)
	}

	return &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		PluginAPIKey: os.Getenv("PLUGIN_API_KEY"),
		OllamaURL:    os.Getenv("OLLAMA_URL"),
		LLMModel:     os.Getenv("LLM_MODEL"),
	}, nil
}
