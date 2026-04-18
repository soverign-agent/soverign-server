// Package jwt provides JWT token generation and validation using RS256.
package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	AccessExpiry time.Time
	RefreshExpiry time.Time
}

// Claims defines custom JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	TokenType string `json:"token_type"`
}

// Manager handles JWT token lifecycle.
type Manager struct {
	privateKey *rsa.PrivateKey
	issuer     string
	accessExp  time.Duration
	refreshExp time.Duration
}

// NewManager creates a JWT manager with the given private key path.
func NewManager(privateKeyPath, issuer string, accessExp, refreshExp time.Duration) (*Manager, error) {
	key, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}
	return &Manager{
		privateKey: key,
		issuer:     issuer,
		accessExp:  accessExp,
		refreshExp: refreshExp,
	}, nil
}

// GenerateTokenPair creates an access token and refresh token for a user.
func (m *Manager) GenerateTokenPair(userID, tenantID, role, email string) (*TokenPair, error) {
	now := time.Now()

	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.issuer},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExp)),
			ID:        uuid.Must(uuid.NewRandom()).String(),
		},
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		Email:     email,
		TokenType: "access",
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims).SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.issuer},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExp)),
			ID:        uuid.Must(uuid.NewRandom()).String(),
		},
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		Email:     email,
		TokenType: "refresh",
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims).SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		AccessExpiry:  now.Add(m.accessExp),
		RefreshExpiry: now.Add(m.refreshExp),
	}, nil
}

// Parse validates a token string and returns its claims.
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &m.privateKey.PublicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// GenerateAccessToken creates a new access token from existing claims (refresh flow).
func (m *Manager) GenerateAccessToken(userID, tenantID, role, email string) (string, time.Time, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.issuer},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExp)),
			ID:        uuid.Must(uuid.NewRandom()).String(),
		},
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		Email:     email,
		TokenType: "access",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(m.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign access token: %w", err)
	}
	return token, now.Add(m.accessExp), nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		key2, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		rsaKey, ok := key2.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}
	return key, nil
}
