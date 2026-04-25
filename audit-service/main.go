// Package main is the audit-service entry point.
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

	"sovereign-ai-compliance/audit-service/internal/client"
	"sovereign-ai-compliance/audit-service/internal/config"
	"sovereign-ai-compliance/audit-service/internal/grpcserver"
	"sovereign-ai-compliance/audit-service/internal/logic"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/scoring"
	"sovereign-ai-compliance/audit-service/temporal"

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

	// Wire dependencies
	repository := repo.NewSQLRepository(db)
	calculator := scoring.NewCalculator(c.Risk)
	auditLogic := logic.NewAuditLogic(repository, calculator)

	// Create notification-service gRPC client
	var notificationClient temporal.NotificationServiceClient
	if c.Notification.Addr != "" {
		notifConn, err := client.DialNotification(c.Notification.Addr, c.Notification.Insecure, c.Notification.TLSCertFile)
		if err != nil {
			logx.Must(fmt.Errorf("failed to dial notification-service: %w", err))
		}
		defer notifConn.Close()
		notificationClient = client.NewNotificationClient(notifConn, repository)
	}

	// Create Temporal client and start worker
	temporalClient, err := temporal.NewClient(c.Temporal, repository, auditLogic, calculator, notificationClient)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create temporal client: %w", err))
	}
	defer temporalClient.Stop()

	// Start Temporal worker in background
	go func() {
		err := temporalClient.Start()
		if err != nil {
			logx.Errorf("Failed to start Temporal worker: %v", err)
		}
	}()

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
	grpcSrv := grpcserver.NewServer(auditLogic, repository, temporalClient)
	grpcSrv.Register(grpcServer)
	reflection.Register(grpcServer)

	go func() {
		logx.Infof("Starting audit-service gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logx.Errorf("gRPC server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("Shutting down audit-service gRPC server...")
	grpcServer.GracefulStop()
}
