// Package main is the doc-service entry point.
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

	"sovereign-ai-compliance/doc-service/internal/client"
	"sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/generator"
	"sovereign-ai-compliance/doc-service/internal/grpcserver"
	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/repo"

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
	conf.MustLoad(*configFile, &c, conf.UseEnv())

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

	clients, err := client.NewClients(c.Clients)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create downstream clients: %w", err))
	}
	defer clients.Close()

	downstreamClients := &generator.DownstreamClients{
		OrgClient:   clients.OrgClient,
		AuditClient: clients.AuditClient,
		RAGClient:   clients.RAGClient,
	}

	documentLogic := logic.NewDocumentLogic(repository, c.LLM, c.Export, logger, downstreamClients)
	versionLogic := logic.NewVersionLogic(repository, logger)
	exportLogic := logic.NewExportLogic(repository, c.Export, logger)

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
	grpcSrv := grpcserver.NewServer(documentLogic, versionLogic, exportLogic)
	grpcSrv.Register(grpcServer)
	reflection.Register(grpcServer)

	go func() {
		logx.Infof("Starting doc-service gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logx.Errorf("gRPC server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("Shutting down doc-service gRPC server...")
	grpcServer.GracefulStop()
}
