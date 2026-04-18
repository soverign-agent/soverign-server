// Package config holds the API gateway configuration.
package config

import "github.com/zeromicro/go-zero/rest"

// Config defines the API gateway configuration.
type Config struct {
	rest.RestConf
	Auth            AuthConfig
	Upstream        UpstreamConfig
	PublicEndpoints []string
}

// AuthConfig holds JWT authentication settings.
type AuthConfig struct {
	PublicKeyPath string
}

// UpstreamConfig holds backend service URLs.
type UpstreamConfig struct {
	Auth         string
	Org          string
	Repo         string
	RAG          string
	Doc          string
	Audit        string
	Agent        string
	Notification string
}
