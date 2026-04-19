// Package analysis implements static code analysis for detecting AI usage and sensitive data.
package analysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AIDetection represents a detected AI model usage.
type AIDetection struct {
	Package string `json:"package"`
	Import  string `json:"import"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// DataFlow represents a detected data flow path.
type DataFlow struct {
	Source      string `json:"source"`       // Where data comes from (e.g., user input, API)
	Sink        string `json:"sink"`         // Where data goes (e.g., LLM input, external API)
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
}

// SensitiveData represents detected sensitive data handling.
type SensitiveData struct {
	Type        string `json:"type"` // e.g., PII, API key, credentials
	Location    string `json:"location"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
}

// AnalysisResult contains the results of static analysis.
type AnalysisResult struct {
	AIDetections     []AIDetection     `json:"ai_detections"`
	DataFlows        []DataFlow        `json:"data_flows"`
	SensitiveDetections []SensitiveData `json:"sensitive_detections"`
	TotalFiles       int               `json:"total_files"`
	ScannedFiles     int               `json:"scanned_files"`
}

// Common AI/LLM imports to detect
var commonAIImports = []string{
	// OpenAI
	"github.com/sashabaranov/go-openai",
	"github.com/openai",
	"openai.com",
	// Anthropic
	"github.com/anthropics/anthropic-sdk-go",
	// Google Gemini
	"github.com/google/generative-ai-go",
	"google.golang.org/genai",
	// Cohere
	"github.com/cohere-ai/cohere-go",
	// Hugging Face
	"github.com/huggingface/go-transformers",
	// LangChain Go
	"github.com/tmc/langchaingo",
	// Llama Go
	"github.com/go-skynet/LlamaGo",
	// Ollama Go
	"github.com/ollama/ollama",
	// AWS Bedrock
	"github.com/aws/aws-sdk-go-v2/service/bedrock",
	// Azure OpenAI
	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai",
}

// Common sensitive data patterns
var sensitiveDataPatterns = []struct {
	Type        string
	Keywords    []string
	Description string
}{
	{
		Type:        "API Key",
		Keywords:    []string{"api_key", "apikey", "api-key", "token", "secret"},
		Description: "Potential API key or secret in code",
	},
	{
		Type:        "PII",
		Keywords:    []string{"email", "phone", "ssn", "social security", "creditcard", "credit_card"},
		Description: "Potential personally identifiable information",
	},
	{
		Type:        "Credentials",
		Keywords:    []string{"password", "username:password", "login", "auth"},
		Description: "Hardcoded credentials detected",
	},
}

// AnalyzeDirectory performs static analysis on a cloned repository directory.
func AnalyzeDirectory(root string) (*AnalysisResult, error) {
	result := &AnalysisResult{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		result.TotalFiles++

		// Skip directories and non-code files
		if info.IsDir() {
			// Skip .git directory
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			// Skip node_modules
			if info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			// Skip vendor directory
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan source code files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".js" && ext != ".ts" && ext != ".py" && ext != ".java" && ext != ".rs" {
			return nil
		}

		result.ScannedFiles++
		return analyzeFile(path, result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func analyzeFile(path string, result *AnalysisResult) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lowerLine := strings.ToLower(line)

		// Check for AI imports
		for _, aiImport := range commonAIImports {
			if strings.Contains(line, aiImport) {
				result.AIDetections = append(result.AIDetections, AIDetection{
					Package: extractPackage(line),
					Import:  aiImport,
					File:    path,
					Line:    lineNum,
				})
			}
		}

		// Check for sensitive data patterns
		for _, pattern := range sensitiveDataPatterns {
			for _, keyword := range pattern.Keywords {
				if strings.Contains(lowerLine, keyword) && !isComment(line) {
					result.SensitiveDetections = append(result.SensitiveDetections, SensitiveData{
						Type:        pattern.Type,
						Location:    fmt.Sprintf("%s:%d", path, lineNum),
						File:        path,
						Line:        lineNum,
						Description: pattern.Description,
					})
				}
			}
		}

		// Look for potential data flows - user input to AI calls
		if containsUserInput(line) && containsAICall(line) && !isComment(line) {
			result.DataFlows = append(result.DataFlows, DataFlow{
				Source:      "User Input",
				Sink:        "AI Model",
				File:        path,
				Line:        lineNum,
				Description: "User input flows directly to AI model",
			})
		}
	}

	return scanner.Err()
}

func extractPackage(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "package ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

func isComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
}

func containsUserInput(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "input") ||
		strings.Contains(lower, "request.body") ||
		strings.Contains(lower, "query") ||
		strings.Contains(lower, "param")
}

func containsAICall(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "openai") ||
		strings.Contains(lower, "completion") ||
		strings.Contains(lower, "generate") ||
		strings.Contains(lower, "chat") ||
		strings.Contains(lower, "gemini") ||
		strings.Contains(lower, "anthropic") ||
		strings.Contains(lower, "ollama")
}

// ToJSON converts the analysis result to JSON bytes.
func (a *AnalysisResult) ToJSON() ([]byte, error) {
	return json.Marshal(a)
}
