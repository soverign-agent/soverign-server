// Package main is the agent-orchestrator entry point.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"sovereign-ai-compliance/agent-orchestrator/internal/client"
	"sovereign-ai-compliance/agent-orchestrator/internal/config"
	"sovereign-ai-compliance/agent-orchestrator/internal/grpcserver"
	"sovereign-ai-compliance/agent-orchestrator/internal/orchestrator"
	"sovereign-ai-compliance/shared/eval"
	"sovereign-ai-compliance/shared/llm"
	"sovereign-ai-compliance/shared/metrics"
	sharedtemporal "sovereign-ai-compliance/shared/temporal"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

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

	logger, err := zap.NewProduction()
	if err != nil {
		logx.Must(fmt.Errorf("failed to create logger: %w", err))
	}
	defer logger.Sync()

	metricsReg := metrics.NewRegistry()
	recorder, err := metrics.NewRecorder(metricsReg)
	if err != nil {
		logx.Must(fmt.Errorf("failed to init metrics recorder: %w", err))
	}

	llmClient := buildLLMClient(c, logger, recorder)

	evalPipeline, err := eval.NewPipeline(recorder, eval.DefaultThresholds())
	if err != nil {
		logx.Must(fmt.Errorf("failed to init eval pipeline: %w", err))
	}

	downstreamClients, err := client.NewClients(c.Clients)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create downstream clients: %w", err))
	}
	defer downstreamClients.Close()

	// Create Temporal client with the shared tenant propagator so the tenant
	// context flows from inbound gRPC calls into workflow / activity Go-context.
	// Without it, downstream RLS-protected gRPC calls invoked from activities
	// would lose their tenant ID and fail authorization.
	temporalClient, err := temporalclient.Dial(temporalclient.Options{
		HostPort:           c.Temporal.HostPort,
		Namespace:          c.Temporal.Namespace,
		ContextPropagators: []workflow.ContextPropagator{sharedtemporal.NewTenantPropagator()},
	})
	if err != nil {
		logx.Must(fmt.Errorf("failed to create temporal client: %w", err))
	}
	defer temporalClient.Close()

	// Register orchestrator workflows and activities on a worker bound to the
	// orchestrator task queue. The worker shares the Temporal client, so all
	// dispatched workflows inherit the tenant propagator above.
	orchestratorWorker := worker.New(temporalClient, orchestrator.OrchestratorTaskQueue, worker.Options{})
	orchestratorWorker.RegisterWorkflow(orchestrator.DocumentGenerationWorkflow)
	orchestratorWorker.RegisterWorkflow(orchestrator.AuditWorkflow)
	orchestratorWorker.RegisterWorkflow(orchestrator.SearchKnowledgeWorkflow)
	orchestratorWorker.RegisterActivity(orchestrator.NewActivities(downstreamClients))

	if err := orchestratorWorker.Start(); err != nil {
		logx.Must(fmt.Errorf("failed to start temporal worker: %w", err))
	}
	defer orchestratorWorker.Stop()

	orc := orchestrator.NewSupervisor(downstreamClients, logger, c.LLM, llmClient, evalPipeline, temporalClient)

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
	grpcSrv := grpcserver.NewServer(orc)
	grpcSrv.Register(grpcServer)
	reflection.Register(grpcServer)

	go func() {
		logx.Infof("Starting agent-orchestrator gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logx.Errorf("gRPC server error: %v", err)
		}
	}()

	metricsPort := c.Metrics.Port
	if metricsPort == 0 {
		metricsPort = 9090
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.NewHandler(metricsReg))
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", metricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logx.Infof("Starting agent-orchestrator metrics server on %s", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Errorf("metrics server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("Shutting down agent-orchestrator...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logx.Errorf("metrics server shutdown: %v", err)
	}
	grpcServer.GracefulStop()
}

// buildLLMClient resolves the LLM API key from env vars (overriding any
// placeholder in config), then constructs the streaming-capable client. We
// require OPENAI_API_KEY in non-development environments to avoid silently
// running with a misconfigured secret.
func buildLLMClient(c config.Config, logger *zap.Logger, rec *metrics.Recorder) llm.Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = c.LLM.APIKey
	}
	if apiKey == "" && os.Getenv("APP_ENV") == "production" {
		logx.Must(fmt.Errorf("OPENAI_API_KEY is required for production startup; set it in env"))
	}

	provider := llm.Provider(c.LLM.Provider)
	if provider == "" {
		provider = llm.ProviderOpenAI
	}

	timeoutSeconds := int(c.LLM.Timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}

	return llm.NewOpenAIClientWithRecorder(llm.Config{
		Provider:    provider,
		APIKey:      apiKey,
		BaseURL:     c.LLM.BaseURL,
		Model:       c.LLM.Model,
		Timeout:     timeoutSeconds,
		MaxTokens:   c.LLM.MaxTokens,
		Temperature: c.LLM.Temperature,
	}, logger, rec)
}
