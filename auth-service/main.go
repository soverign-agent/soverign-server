// Package main is the auth-service entry point.
package main

import (
	"flag"
	"fmt"

	"sovereign-ai-compliance/auth-service/internal/config"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// TODO: wire repository and JWT manager, then create handler and register routes
	// authHandler := handler.NewAuthHandler(...)
	// handler.RegisterRoutes(server, authHandler)

	fmt.Printf("Starting auth-service at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
