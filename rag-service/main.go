// Package main is the rag-service entry point.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	"sovereign-ai-compliance/rag-service/internal/config"
	"sovereign-ai-compliance/rag-service/internal/handler"
	"sovereign-ai-compliance/rag-service/internal/logic"
	"sovereign-ai-compliance/rag-service/processing"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"

	_ "github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"go.uber.org/zap"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Open database connection
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Database, c.Database.SSLMode,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logx.Must(fmt.Errorf("failed to open database: %w", err))
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logx.Must(fmt.Errorf("failed to ping database: %w", err))
	}

	// Initialize zap logger
	logger, err := zap.NewProduction()
	if err != nil {
		logx.Must(fmt.Errorf("failed to create logger: %w", err))
	}
	defer logger.Sync()

	// Initialize LLM client
	llmConfig := llm.Config{
		Provider:    c.LLM.Provider,
		APIKey:      c.LLM.APIKey,
		BaseURL:     c.LLM.BaseURL,
		Model:       c.LLM.EmbeddingModel,
		Timeout:     c.LLM.Timeout,
		MaxTokens:   c.LLM.MaxTokens,
		Temperature: c.LLM.Temperature,
	}
	llmClient, err := llm.NewClient(llmConfig, logger)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create LLM client: %w", err))
	}

	// Wire dependencies
	repository := repo.NewSQLRepository(db)
	chunker := processing.NewChunker(c.Chunking.DefaultChunkSize, c.Chunking.DefaultChunkOverlap)
	extractor := processing.NewExtractor()
	processor := processing.NewProcessor(extractor, chunker)
	documentsLogic := logic.NewDocumentsLogic(c, repository, processor, llmClient, logger)
	searchLogic := logic.NewSearchLogic(repository, llmClient, logger)
	statsLogic := logic.NewStatsLogic(repository, logger)

	// Create handlers
	documentsHandler := handler.NewDocumentsHandler(documentsLogic)
	searchHandler := handler.NewSearchHandler(searchLogic)
	statsHandler := handler.NewStatsHandler(statsLogic)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterRoutes(server, documentsHandler, searchHandler, statsHandler)

	fmt.Printf("Starting rag-service at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
