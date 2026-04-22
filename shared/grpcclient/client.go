// Package grpcclient provides reusable gRPC client connection helpers.
package grpcclient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Connect establishes a gRPC connection with optional TLS.
// If certFile is empty and insecure is true, it uses insecure credentials.
func Connect(addr, certFile string, insecureFlag bool) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if certFile != "" {
		creds, err := credentials.NewClientTLSFromFile(certFile, "")
		if err != nil {
			return nil, fmt.Errorf("load client TLS: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else if insecureFlag {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, fmt.Errorf("either cert_file or insecure=true is required")
	}
	return grpc.NewClient(addr, opts...)
}
