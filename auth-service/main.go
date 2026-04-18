// Package main is the auth-service entry point.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	"sovereign-ai-compliance/auth-service/internal/config"
	"sovereign-ai-compliance/auth-service/internal/handler"
	"sovereign-ai-compliance/auth-service/internal/logic"
	"sovereign-ai-compliance/auth-service/jwt"
	"sovereign-ai-compliance/auth-service/repo"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	_ "github.com/lib/pq"
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

	// Wire dependencies
	repository := repo.NewSQLRepository(db)
	jwtManager, err := jwt.NewManager(
		c.Auth.PrivateKeyPath,
		c.Auth.Issuer,
		c.Auth.AccessExpiry,
		c.Auth.RefreshExpiry,
	)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create JWT manager: %w", err))
	}

	authLogic := logic.NewAuth(repository, jwtManager)
	authHandler := handler.NewAuthHandler(authLogic)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterRoutes(server, authHandler)

	fmt.Printf("Starting auth-service at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
