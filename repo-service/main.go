// Package main is the repo-service entry point.
package main

import (
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	"sovereign-ai-compliance/repo-service/internal/config"
	"sovereign-ai-compliance/repo-service/internal/handler"
	"sovereign-ai-compliance/repo-service/internal/logic"
	"sovereign-ai-compliance/repo-service/repo"
	"sovereign-ai-compliance/shared/security"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	_ "github.com/lib/pq"
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

	// Initialize encryption for credentials
	// Decode encryption key from hex
	keyBytes, err := hex.DecodeString(c.EncryptionKey)
	if err != nil {
		logx.Must(fmt.Errorf("invalid encryption key (must be hex-encoded): %w", err))
	}
	encryption, err := security.NewAESGCMEncryption(keyBytes)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create encryption: %w", err))
	}
	// Zero the key bytes after use to avoid leaving in memory
	for i := range keyBytes {
		keyBytes[i] = 0
	}

	// Initialize zap logger
	logger, err := zap.NewProduction()
	if err != nil {
		logx.Must(fmt.Errorf("failed to create logger: %w", err))
	}
	defer logger.Sync()

	// Wire dependencies
	repository := repo.NewSQLRepository(db)
	repoLogic := logic.NewRepositoryLogic(c, repository, encryption, logger)
	repoHandler := handler.NewRepositoryHandler(repoLogic)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterRoutes(server, repoHandler)

	fmt.Printf("Starting repo-service at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
