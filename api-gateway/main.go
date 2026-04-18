// Package main is the API gateway entry point.
package main

import (
	"flag"
	"fmt"

	"sovereign-ai-compliance/api-gateway/internal/config"
	"sovereign-ai-compliance/api-gateway/internal/handler"
	"sovereign-ai-compliance/api-gateway/internal/svc"

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

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting API gateway at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
