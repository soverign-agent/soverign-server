// Package main is the monitoring-service entry point.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"sovereign-ai-compliance/monitoring-service/internal/config"
	"sovereign-ai-compliance/monitoring-service/internal/grpcserver"
	"sovereign-ai-compliance/monitoring-service/internal/logic"
	"sovereign-ai-compliance/monitoring-service/internal/promql"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	logger, err := zap.NewProduction()
	if err != nil {
		logx.Must(fmt.Errorf("init logger: %w", err))
	}
	defer func() { _ = logger.Sync() }()

	querier, err := promql.NewClient(c.Prometheus.URL)
	if err != nil {
		logx.Must(fmt.Errorf("init promql client: %w", err))
	}
	service := logic.NewService(querier, logger)

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
	grpcSrv := grpcserver.NewServer(service)
	grpcSrv.Register(grpcServer)
	reflection.Register(grpcServer)

	go func() {
		logx.Infof("Starting monitoring-service gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logx.Errorf("gRPC server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("Shutting down monitoring-service gRPC server...")
	grpcServer.GracefulStop()
}
