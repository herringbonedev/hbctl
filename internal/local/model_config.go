package local

import (
	"os"
	"strings"
)

const DefaultFingerprintTunerModel = "qwen2.5-coder:7b"

// ResolveFingerprintTunerModel is the single source of truth used by hbctl start,
// hbctl model show/use, Ollama pull checks, and docker compose env injection.
// Priority:
//  1. Process FINGERPRINT_TUNER_LLM_MODEL
//  2. Process OLLAMA_MODEL
//  3. .env FINGERPRINT_TUNER_LLM_MODEL
//  4. .env OLLAMA_MODEL
//  5. Stable default for structured JSON tuning
func ResolveFingerprintTunerModel() string {
	if model := strings.TrimSpace(os.Getenv("FINGERPRINT_TUNER_LLM_MODEL")); model != "" {
		return model
	}
	if model := strings.TrimSpace(os.Getenv("OLLAMA_MODEL")); model != "" {
		return model
	}
	if model := readDotEnvValue("FINGERPRINT_TUNER_LLM_MODEL"); model != "" {
		return model
	}
	if model := readDotEnvValue("OLLAMA_MODEL"); model != "" {
		return model
	}
	return DefaultFingerprintTunerModel
}

func readDotEnvValue(key string) string {
	content, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		value = strings.Trim(value, `"'`)
		return value
	}
	return ""
}
