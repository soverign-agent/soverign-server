// Package main is the rag-service entry point.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sovereign-ai-compliance/rag-service/internal/config"
	"sovereign-ai-compliance/rag-service/internal/grpcserver"
	"sovereign-ai-compliance/rag-service/internal/logic"
	"sovereign-ai-compliance/rag-service/internal/orchestrator"
	"sovereign-ai-compliance/rag-service/processing"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"

	_ "github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
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
	chatModel := c.LLM.ChatModel
	if chatModel == "" {
		chatModel = "gpt-4o-mini"
	}
	llmConfig := llm.Config{
		Provider:    c.LLM.Provider,
		APIKey:      c.LLM.APIKey,
		BaseURL:     c.LLM.BaseURL,
		Model:       chatModel,
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
	searchLogic := logic.NewSearchLogic(repository, llmClient, c.LLM.EmbeddingModel, logger)
	statsLogic := logic.NewStatsLogic(repository, logger)

	// Build the agentic RAG pipeline that powers the streaming /chat endpoint.
	// The pipeline is wrapped behind logic.ChatPipeline so the chat logic can
	// be tested without a real LLM/vector store.
	chatPipeline := orchestrator.NewPipeline(
		orchestrator.Dependencies{
			LLMClient:      llmClient,
			EmbeddingModel: c.LLM.EmbeddingModel,
			Repository:     repository,
		},
		logger,
		orchestrator.DefaultGuardrailConfig(),
		10,
	)
	chatLogic := logic.NewChatLogic(repository, chatPipeline, logger, logic.DefaultChatRateLimiterConfig())

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", c.GRPC.Port)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logx.Must(fmt.Errorf("failed to listen on %s: %w", grpcAddr, err))
	}

	var grpcOpts []grpc.ServerOption
	if c.GRPC.TLSCertFile != "" && c.GRPC.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(c.GRPC.TLSCertFile, c.GRPC.TLSKeyFile)
		if err != nil {
			logx.Must(fmt.Errorf("load TLS credentials: %w", err))
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	} else if !c.GRPC.Insecure {
		logx.Must(fmt.Errorf("gRPC TLS config required; set tls_cert_file/tls_key_file or insecure=true for local dev"))
	} else {
		grpcOpts = append(grpcOpts, grpc.Creds(insecure.NewCredentials()))
	}
	grpcServer := grpc.NewServer(grpcOpts...)
	grpcSrv := grpcserver.NewServer(documentsLogic, searchLogic, statsLogic, chatLogic)
	grpcSrv.Register(grpcServer)
	reflection.Register(grpcServer)

	go func() {
		logx.Infof("Starting rag-service gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logx.Errorf("gRPC server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("Shutting down rag-service gRPC server...")
	grpcServer.GracefulStop()
}
