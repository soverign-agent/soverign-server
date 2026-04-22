// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// claimsKey is the context key for JWT claims.
type claimsKey struct{}

var ck = claimsKey{}

// JWTAuth returns a middleware that validates JWT tokens using RS256.
// It skips validation for endpoints listed in publicEndpoints.
func JWTAuth(publicKeyPath string, publicEndpoints []string) func(http.Handler) http.Handler {
	publicKey := mustLoadPublicKey(publicKeyPath)

	publicPathSet := make(map[string]struct{}, len(publicEndpoints))
	for _, p := range publicEndpoints {
		publicPathSet[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := publicPathSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w, "invalid authorization header format")
				return
			}

			token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return publicKey, nil
			})
			if err != nil || !token.Valid {
				unauthorized(w, "invalid token")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				unauthorized(w, "invalid token claims")
				return
			}

			ctx := context.WithValue(r.Context(), ck, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) jwt.MapClaims {
	v := ctx.Value(ck)
	if v == nil {
		return nil
	}
	return v.(jwt.MapClaims)
}

func unauthorized(w http.ResponseWriter, message string) {
	log.Printf("JWT auth failed: %s", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    401,
		"message": message,
	})
}

func mustLoadPublicKey(path string) *rsa.PublicKey {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read JWT public key: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		log.Fatalf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse public key: %v", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		log.Fatalf("public key is not RSA")
	}

	return rsaPub
}
