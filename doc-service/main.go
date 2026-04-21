// Package main is the doc-service entry point.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	"sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/handler"
	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/repo"

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

	// Wire dependencies
	repository := repo.NewSQLRepository(db)
	documentLogic := logic.NewDocumentLogic(repository, c.LLM, c.Export, logger)
	versionLogic := logic.NewVersionLogic(repository, logger)
	exportLogic := logic.NewExportLogic(repository, c.Export, logger)

	// Create handlers
	documentHandler := handler.NewDocumentHandler(documentLogic, versionLogic)
	exportHandler := handler.NewExportHandler(exportLogic)

	// Create server
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// Register routes
	handler.RegisterRoutes(server, documentHandler, exportHandler)

	fmt.Printf("Starting doc-service at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
