package logic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/auth-service/jwt"
	"sovereign-ai-compliance/auth-service/model"
	"sovereign-ai-compliance/auth-service/password"
)

// mockRepo implements repo.Repository for testing.
type mockRepo struct {
	users               map[uuid.UUID]*model.User
	usersByEmail        map[string]*model.User
	refreshTokens       map[uuid.UUID]*model.RefreshToken
	resetTokens         map[uuid.UUID]*model.PasswordResetToken
	storedRefreshTokens []*model.RefreshToken
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:        make(map[uuid.UUID]*model.User),
		usersByEmail: make(map[string]*model.User),
		refreshTokens: make(map[uuid.UUID]*model.RefreshToken),
		resetTokens:   make(map[uuid.UUID]*model.PasswordResetToken),
	}
}

func (m *mockRepo) GetUserByEmail(_ context.Context, tenantID uuid.UUID, email string) (*model.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok || u.TenantID != tenantID {
		return nil, context.DeadlineExceeded
	}
	return u, nil
}

func (m *mockRepo) GetUserByID(_ context.Context, tenantID, userID uuid.UUID) (*model.User, error) {
	u, ok := m.users[userID]
	if !ok || u.TenantID != tenantID {
		return nil, context.DeadlineExceeded
	}
	return u, nil
}

func (m *mockRepo) CreateUser(_ context.Context, user *model.User) error {
	m.users[user.ID] = user
	m.usersByEmail[user.Email] = user
	return nil
}

func (m *mockRepo) UpdateUser(_ context.Context, user *model.User) error {
	m.users[user.ID] = user
	m.usersByEmail[user.Email] = user
	return nil
}

func (m *mockRepo) UpdateLastLogin(_ context.Context, _, _ uuid.UUID) error { return nil }

func (m *mockRepo) StoreRefreshToken(_ context.Context, token *model.RefreshToken) error {
	m.refreshTokens[token.ID] = token
	m.storedRefreshTokens = append(m.storedRefreshTokens, token)
	return nil
}

func (m *mockRepo) GetRefreshToken(_ context.Context, userID uuid.UUID, tokenHash string) (*model.RefreshToken, error) {
	for _, t := range m.refreshTokens {
		if t.UserID == userID && t.TokenHash == tokenHash && !t.Revoked {
			return t, nil
		}
	}
	return nil, context.DeadlineExceeded
}

func (m *mockRepo) RevokeRefreshToken(_ context.Context, id uuid.UUID) error {
	if t, ok := m.refreshTokens[id]; ok {
		t.Revoked = true
	}
	return nil
}

func (m *mockRepo) RevokeAllUserTokens(_ context.Context, userID uuid.UUID) error {
	for _, t := range m.refreshTokens {
		if t.UserID == userID {
			t.Revoked = true
		}
	}
	return nil
}

func (m *mockRepo) StorePasswordResetToken(_ context.Context, token *model.PasswordResetToken) error {
	m.resetTokens[token.ID] = token
	return nil
}

func (m *mockRepo) GetPasswordResetToken(_ context.Context, tokenHash string) (*model.PasswordResetToken, error) {
	for _, t := range m.resetTokens {
		if t.TokenHash == tokenHash && !t.Used {
			return t, nil
		}
	}
	return nil, context.DeadlineExceeded
}

func (m *mockRepo) MarkResetTokenUsed(_ context.Context, id uuid.UUID) error {
	if t, ok := m.resetTokens[id]; ok {
		t.Used = true
	}
	return nil
}

func setupTestAuth(t *testing.T) (*Auth, *mockRepo) {
	t.Helper()
	keyPath := writeTestPrivateKey(t)
	jwtManager, err := jwt.NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create jwt manager: %v", err)
	}
	r := newMockRepo()
	return NewAuth(r, jwtManager), r
}

func createTestUser(repo *mockRepo, email, plaintextPassword, role string, active bool) *model.User {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.Must(uuid.NewRandom())
	hash, _ := password.Hash(plaintextPassword)
	user := &model.User{
		ID:           userID,
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		IsActive:     active,
	}
	repo.users[userID] = user
	repo.usersByEmail[email] = user
	return user
}

func TestLogin_Success(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	pair, safeUser, err := auth.Login(context.Background(), user.TenantID, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected tokens")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access_token and refresh_token must be different")
	}
	if safeUser.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", safeUser.Email)
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	_, _, err := auth.Login(context.Background(), user.TenantID, "test@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	auth, _ := setupTestAuth(t)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	_, _, err := auth.Login(context.Background(), tenantID, "nonexistent@example.com", "password123")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_InactiveUser(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "inactive@example.com", "password123", "admin", false)

	_, _, err := auth.Login(context.Background(), user.TenantID, "inactive@example.com", "password123")
	if err == nil {
		t.Fatal("expected error for inactive user")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_SameErrorForMissingAndWrongPassword(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)
	tenantID := user.TenantID

	_, _, err1 := auth.Login(context.Background(), tenantID, "missing@example.com", "password123")
	_, _, err2 := auth.Login(context.Background(), tenantID, "test@example.com", "wrongpassword")

	if err1.Error() != err2.Error() {
		t.Errorf("expected same error message for missing user and wrong password, got %q vs %q", err1.Error(), err2.Error())
	}
}

func TestRefreshToken_Success(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	pair, _, err := auth.Login(context.Background(), user.TenantID, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	newPair, err := auth.RefreshToken(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatal("expected new tokens")
	}
	if newPair.AccessToken == newPair.RefreshToken {
		t.Fatal("access_token and refresh_token must be different")
	}
	// Old refresh token should be revoked after rotation
	_, err = auth.RefreshToken(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error when reusing old refresh token")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	auth, _ := setupTestAuth(t)

	_, err := auth.RefreshToken(context.Background(), "invalid.token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestLogout(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	pair, _, _ := auth.Login(context.Background(), user.TenantID, "test@example.com", "password123")
	if err := auth.Logout(context.Background(), user.ID); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Refresh token should be invalid after logout
	_, err := auth.RefreshToken(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error after logout")
	}
}

func TestValidateToken(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	pair, _, _ := auth.Login(context.Background(), user.TenantID, "test@example.com", "password123")

	uid, tid, role, err := auth.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if uid != user.ID.String() {
		t.Errorf("expected %s, got %s", user.ID.String(), uid)
	}
	if tid != user.TenantID.String() {
		t.Errorf("expected %s, got %s", user.TenantID.String(), tid)
	}
	if role != "admin" {
		t.Errorf("expected admin, got %s", role)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	auth, _ := setupTestAuth(t)

	_, _, _, err := auth.ValidateToken("invalid")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestHasPermission(t *testing.T) {
	auth, _ := setupTestAuth(t)
	if !auth.HasPermission("admin", "user", "create") {
		t.Error("expected admin to have permission")
	}
	if auth.HasPermission("reviewer", "user", "delete") {
		t.Error("expected reviewer to NOT have permission")
	}
}

func TestRequestPasswordReset(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	if err := auth.RequestPasswordReset(context.Background(), user.TenantID, "test@example.com"); err != nil {
		t.Fatalf("request reset failed: %v", err)
	}
}

func TestRequestPasswordReset_NonexistentEmail(t *testing.T) {
	auth, _ := setupTestAuth(t)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	// Should not leak email existence - returns nil
	if err := auth.RequestPasswordReset(context.Background(), tenantID, "missing@example.com"); err != nil {
		t.Errorf("expected no error for nonexistent email, got %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	_ = auth.RequestPasswordReset(context.Background(), user.TenantID, "test@example.com")

	// Get the stored token hash and reconstruct raw token for test
	var rawToken string
	for _, tkn := range repo.resetTokens {
		rawToken = tkn.TokenHash // In real scenario this would be the raw token sent via email
		_ = rawToken
		break
	}

	// Use a known token for testing by directly storing one
	testToken := "test-reset-token-123"
	tokenHash := hashToken(testToken)
	resetTkn := &model.PasswordResetToken{
		ID:        uuid.Must(uuid.NewRandom()),
		UserID:    user.ID,
		TenantID:  user.TenantID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	repo.resetTokens[resetTkn.ID] = resetTkn

	if err := auth.ResetPassword(context.Background(), testToken, "newpassword123"); err != nil {
		t.Fatalf("reset password failed: %v", err)
	}

	// Verify new password works
	_, _, err := auth.Login(context.Background(), user.TenantID, "test@example.com", "newpassword123")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	auth, _ := setupTestAuth(t)

	if err := auth.ResetPassword(context.Background(), "invalid-token", "newpassword123"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestMe(t *testing.T) {
	auth, repo := setupTestAuth(t)
	user := createTestUser(repo, "test@example.com", "password123", "admin", true)

	safe, err := auth.Me(context.Background(), user.TenantID, user.ID)
	if err != nil {
		t.Fatalf("me failed: %v", err)
	}
	if safe.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", safe.Email)
	}
	if safe.Role != "admin" {
		t.Errorf("expected admin, got %s", safe.Role)
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	privPath := filepath.Join(dir, "jwt-private.pem")
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	return privPath
}
