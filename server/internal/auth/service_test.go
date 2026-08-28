package auth

import (
	"testing"
	"time"
)

// Mock repository for testing
type mockRepository struct {
	users      map[string]*User
	usersID    map[int64]*User
	resetCodes map[string]*ResetCode
}

func (m *mockRepository) GetByUsername(username string) (*User, error) {
	user, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *mockRepository) GetByID(id int64) (*User, error) {
	user, exists := m.usersID[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *mockRepository) Create(username, passwordHash string, isAdmin bool) (*User, error) {
	user := &User{
		ID:           int64(len(m.users) + 1),
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
	}
	m.users[username] = user
	m.usersID[user.ID] = user
	return user, nil
}

func (m *mockRepository) UpdatePassword(userID int64, newPasswordHash string) error {
	user, exists := m.usersID[userID]
	if !exists {
		return ErrUserNotFound
	}
	user.PasswordHash = newPasswordHash
	return nil
}

func (m *mockRepository) GetAdminCount() (int, error) {
	count := 0
	for _, user := range m.users {
		if user.IsAdmin {
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) StoreResetCode(code string, userID int64, expiresAt time.Time) error {
	m.resetCodes[code] = &ResetCode{
		Code:      code,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *mockRepository) GetResetCode(code string) (*ResetCode, error) {
	resetCode, exists := m.resetCodes[code]
	if !exists {
		return nil, ErrInvalidResetCode
	}
	return resetCode, nil
}

func (m *mockRepository) DeleteResetCode(code string) error {
	delete(m.resetCodes, code)
	return nil
}

func (m *mockRepository) DeleteResetCodesByUserID(userID int64) error {
	for code, rc := range m.resetCodes {
		if rc.UserID == userID {
			delete(m.resetCodes, code)
		}
	}
	return nil
}

// ExpireResetCode is a helper for testing - manually expires a code
func (m *mockRepository) ExpireResetCode(code string) {
	if rc, exists := m.resetCodes[code]; exists {
		rc.ExpiresAt = time.Now().Add(-1 * time.Hour) // 1 hour in the past
	}
}

func (m *mockRepository) DeleteExpiredResetCodes() error {
	now := time.Now()
	for code, rc := range m.resetCodes {
		if now.After(rc.ExpiresAt) {
			delete(m.resetCodes, code)
		}
	}
	return nil
}

func (m *mockRepository) GetUserCount() (int, error) {
	return len(m.users), nil
}

func (m *mockRepository) GetFirstAdmin() (*User, error) {
	var lowest *User
	for _, u := range m.users {
		if !u.IsAdmin {
			continue
		}
		if lowest == nil || u.ID < lowest.ID {
			lowest = u
		}
	}
	if lowest == nil {
		return nil, ErrUserNotFound
	}
	return lowest, nil
}

func (m *mockRepository) ListUsers() ([]*User, error) {
	users := make([]*User, 0, len(m.usersID))
	for _, u := range m.usersID {
		users = append(users, u)
	}
	for i := 0; i < len(users); i++ {
		for j := i + 1; j < len(users); j++ {
			if users[j].ID < users[i].ID {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
	return users, nil
}

func (m *mockRepository) Delete(userID int64) error {
	u, ok := m.usersID[userID]
	if !ok {
		return ErrUserNotFound
	}
	delete(m.usersID, userID)
	delete(m.users, u.Username)
	return nil
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		users:      make(map[string]*User),
		usersID:    make(map[int64]*User),
		resetCodes: make(map[string]*ResetCode),
	}
}

func TestLogin_ValidCredentials_ReturnsJWT(t *testing.T) {
	// Arrange: Create a user with hashed password
	repo := newMockRepository()

	// Hash the password "audiod"
	passwordHash, err := HashPassword("audiod")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// Add user to mock repository
	user := &User{
		ID:           1,
		Username:     "audiod",
		PasswordHash: passwordHash,
		IsAdmin:      true,
	}
	repo.users["audiod"] = user
	repo.usersID[1] = user

	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Attempt login with correct credentials
	token, user, err := service.Login("audiod", "audiod", SessionContext{})

	// Assert: Should return a JWT token and user without error
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if token == "" {
		t.Fatal("expected JWT token, got empty string")
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}

	if user.Username != "audiod" {
		t.Errorf("expected username 'audiod', got '%s'", user.Username)
	}

	// Token should be a non-empty string
	if len(token) < 10 {
		t.Errorf("expected JWT token to be at least 10 characters, got: %d", len(token))
	}
}

func TestLogin_InvalidUsername_ReturnsError(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Attempt login with non-existent user
	token, user, err := service.Login("nonexistent", "password", SessionContext{})

	// Assert: Should return error
	if err == nil {
		t.Fatal("expected error for invalid username, got nil")
	}

	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got: %v", err)
	}

	if token != "" {
		t.Errorf("expected empty token, got: %s", token)
	}

	if user != nil {
		t.Errorf("expected nil user, got: %v", user)
	}
}

func TestLogin_WrongPassword_ReturnsError(t *testing.T) {
	// Arrange: Create a user with hashed password
	repo := newMockRepository()

	passwordHash, err := HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &User{
		ID:           1,
		Username:     "audiod",
		PasswordHash: passwordHash,
		IsAdmin:      true,
	}
	repo.users["audiod"] = user
	repo.usersID[1] = user

	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Attempt login with wrong password
	token, user, err := service.Login("audiod", "wrongpassword", SessionContext{})

	// Assert: Should return error
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}

	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got: %v", err)
	}

	if token != "" {
		t.Errorf("expected empty token, got: %s", token)
	}

	if user != nil {
		t.Errorf("expected nil user, got: %v", user)
	}
}

func TestUpdatePassword_ValidCurrentPassword_UpdatesPassword(t *testing.T) {
	// Arrange: Create a user with known password
	repo := newMockRepository()

	oldPasswordHash, err := HashPassword("oldpassword")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &User{
		ID:           1,
		Username:     "testuser",
		PasswordHash: oldPasswordHash,
		IsAdmin:      false,
	}
	repo.users["testuser"] = user
	repo.usersID[1] = user

	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Update password
	sessionID, err := service.UpdatePassword(1, "oldpassword", "newpassword", SessionContext{})

	// Assert: Should succeed, update the hash, and hand back a fresh session
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if sessionID == "" {
		t.Error("expected a replacement session for the caller, got an empty id")
	}

	// Verify new password works for login
	_, _, err = service.Login("testuser", "newpassword", SessionContext{})
	if err != nil {
		t.Errorf("expected login with new password to succeed, got error: %v", err)
	}

	// Verify old password no longer works
	_, _, err = service.Login("testuser", "oldpassword", SessionContext{})
	if err == nil {
		t.Error("expected login with old password to fail, but it succeeded")
	}
}

// TestRequestPasswordReset_GeneratesValidCode tests that requesting a reset generates a valid code
func TestRequestPasswordReset_GeneratesValidCode(t *testing.T) {
	// Arrange: Create a user
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("oldpass")
	user := &User{
		ID:           1,
		Username:     "alice",
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	repo.users["alice"] = user
	repo.usersID[1] = user

	// Act: Request password reset
	code, err := service.RequestPasswordReset("alice")

	// Assert: Should return a valid reset code
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if code == "" {
		t.Fatal("expected reset code, got empty string")
	}

	if len(code) != 8 {
		t.Errorf("expected 8-character code, got %d characters", len(code))
	}

	// Code should be alphanumeric uppercase
	for _, c := range code {
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
			t.Errorf("expected Base32 characters only, got invalid character: %c", c)
		}
	}
}

// TestRequestPasswordReset_SecondRequest_InvalidatesFirstCode tests that a second request invalidates the first code
func TestRequestPasswordReset_SecondRequest_InvalidatesFirstCode(t *testing.T) {
	// Arrange: Create user and get first reset code
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("oldpass")
	user := &User{
		ID:           1,
		Username:     "alice",
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	repo.users["alice"] = user
	repo.usersID[1] = user

	firstCode, _ := service.RequestPasswordReset("alice")

	// Act: Request reset again
	secondCode, err := service.RequestPasswordReset("alice")

	// Assert: Second code should be different
	if err != nil {
		t.Fatalf("expected no error on second request, got: %v", err)
	}

	if firstCode == secondCode {
		t.Error("expected different codes, got same code twice")
	}

	// First code should no longer be valid
	_, err = service.ConfirmPasswordReset(firstCode, "newpass123")
	if err == nil {
		t.Error("expected first code to be invalid after second request, but it worked")
	}
}

// TestConfirmPasswordReset_ValidCode_SetsNewPassword tests that confirming with a valid code sets the new password
func TestConfirmPasswordReset_ValidCode_SetsNewPassword(t *testing.T) {
	// Arrange: Create user with old password, then get reset code
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("oldpassword")
	user := &User{
		ID:           1,
		Username:     "alice",
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	repo.users["alice"] = user
	repo.usersID[1] = user

	code, _ := service.RequestPasswordReset("alice")

	// Act: Confirm password reset with valid code and new password
	newPassword := "newpassword123"
	resetUser, err := service.ConfirmPasswordReset(code, newPassword)

	// Assert: Should succeed and return user
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resetUser == nil {
		t.Fatal("expected user object, got nil")
	}

	// User should still have same username
	if resetUser.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", resetUser.Username)
	}

	// Should be able to login with new password
	_, _, err = service.Login("alice", newPassword, SessionContext{})
	if err != nil {
		t.Errorf("expected login with new password to work, got error: %v", err)
	}

	// Should NOT be able to login with old password
	_, _, err = service.Login("alice", "oldpassword", SessionContext{})
	if err == nil {
		t.Error("expected login with old password to fail, but it succeeded")
	}

	// Code should be consumed (no longer valid)
	_, err = service.ConfirmPasswordReset(code, "anotherpass")
	if err == nil {
		t.Error("expected code to be invalid after use, but it worked again")
	}
}

// TestConfirmPasswordReset_InvalidCode_Fails tests that an invalid code is rejected
func TestConfirmPasswordReset_InvalidCode_Fails(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Try to confirm with invalid code
	user, err := service.ConfirmPasswordReset("INVALID1", "newpass123")

	// Assert: Should fail
	if err == nil {
		t.Error("expected error for invalid code, got nil")
	}

	if user != nil {
		t.Error("expected nil user for invalid code, got user object")
	}
}

// TestConfirmPasswordReset_ExpiredCode_Fails tests that an expired code is rejected
func TestConfirmPasswordReset_ExpiredCode_Fails(t *testing.T) {
	// Arrange: Create user and reset code
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("oldpass")
	user := &User{
		ID:           1,
		Username:     "alice",
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	repo.users["alice"] = user
	repo.usersID[1] = user

	code, _ := service.RequestPasswordReset("alice")

	// Simulate code expiration by manually expiring it in the repo
	repo.ExpireResetCode(code)

	// Act: Try to confirm with expired code
	user, err := service.ConfirmPasswordReset(code, "newpass123")

	// Assert: Should fail
	if err == nil {
		t.Error("expected error for expired code, got nil")
	}

	if user != nil {
		t.Error("expected nil user for expired code, got user object")
	}
}

// TestGetUserCount_ReturnsCorrectCount tests that GetUserCount returns the correct number of users
func TestGetUserCount_ReturnsCorrectCount(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Check count when no users
	count, err := service.GetUserCount()

	// Assert: Should return 0
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	// Arrange: Add a user
	passwordHash, _ := HashPassword("testpass")
	user := &User{
		ID:           1,
		Username:     "alice",
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	repo.users["alice"] = user
	repo.usersID[1] = user

	// Act: Check count with one user
	count, err = service.GetUserCount()

	// Assert: Should return 1
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

// TestCreateFirstUser_NoUsersExist_CreatesUser tests that CreateFirstUser creates a user when no users exist
func TestCreateFirstUser_NoUsersExist_CreatesUser(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Create first user
	token, user, err := service.CreateFirstUser("alice", "alicepass123", SessionContext{})

	// Assert: Should succeed and return user + token
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("expected user object, got nil")
	}

	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	if !user.IsAdmin {
		t.Error("expected isAdmin to be true for first user")
	}

	if token == "" {
		t.Error("expected JWT token, got empty string")
	}

	// Verify user can login with provided credentials
	_, _, err = service.Login("alice", "alicepass123", SessionContext{})
	if err != nil {
		t.Errorf("expected login to succeed with new credentials, got error: %v", err)
	}
}

// TestCreateFirstUser_UsersExist_ReturnsError tests that CreateFirstUser returns an error if users already exist
func TestCreateFirstUser_UsersExist_ReturnsError(t *testing.T) {
	// Arrange: Create existing user
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("existingpass")
	existingUser := &User{
		ID:           1,
		Username:     "existing",
		PasswordHash: passwordHash,
		IsAdmin:      true,
	}
	repo.users["existing"] = existingUser
	repo.usersID[1] = existingUser

	// Act: Try to create first user when users exist
	token, user, err := service.CreateFirstUser("alice", "alicepass123", SessionContext{})

	// Assert: Should fail
	if err == nil {
		t.Fatal("expected error when users already exist, got nil")
	}

	if token != "" {
		t.Errorf("expected empty token on error, got: %s", token)
	}

	if user != nil {
		t.Errorf("expected nil user on error, got: %v", user)
	}
}

// TestCreateUser_InSetupMode_WithoutAdminFlag_ReturnsError tests that creating user in setup mode requires --admin
func TestCreateUser_InSetupMode_WithoutAdminFlag_ReturnsError(t *testing.T) {
	// Arrange: No users exist (setup mode)
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Try to create user without admin flag
	user, err := service.CreateUser("bob", "bobpass", false)

	// Assert: Should fail
	if err == nil {
		t.Fatal("expected error when creating non-admin user in setup mode, got nil")
	}

	if user != nil {
		t.Errorf("expected nil user on error, got: %v", user)
	}
}

// TestCreateUser_InSetupMode_WithAdminFlag_CreatesAdmin tests that creating user in setup mode with --admin works
func TestCreateUser_InSetupMode_WithAdminFlag_CreatesAdmin(t *testing.T) {
	// Arrange: No users exist (setup mode)
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	// Act: Create admin user
	user, err := service.CreateUser("alice", "alicepass", true)

	// Assert: Should succeed
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}

	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	if !user.IsAdmin {
		t.Error("expected user to be admin")
	}
}

// TestCreateUser_InNormalMode_CreatesNonAdminUser tests creating a non-admin user when admin exists
func TestCreateUser_InNormalMode_CreatesNonAdminUser(t *testing.T) {
	// Arrange: Admin user already exists
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("adminpass")
	adminUser := &User{
		ID:           1,
		Username:     "admin",
		PasswordHash: passwordHash,
		IsAdmin:      true,
	}
	repo.users["admin"] = adminUser
	repo.usersID[1] = adminUser

	// Act: Create non-admin user
	user, err := service.CreateUser("bob", "bobpass", false)

	// Assert: Should succeed
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}

	if user.Username != "bob" {
		t.Errorf("expected username 'bob', got '%s'", user.Username)
	}

	if user.IsAdmin {
		t.Error("expected user to NOT be admin")
	}

	// Verify user can login
	_, _, err = service.Login("bob", "bobpass", SessionContext{})
	if err != nil {
		t.Errorf("expected login to succeed, got error: %v", err)
	}
}

// TestCreateUser_InNormalMode_CreatesAdminUser tests creating an admin user when admin exists
func TestCreateUser_InNormalMode_CreatesAdminUser(t *testing.T) {
	// Arrange: Admin user already exists
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	passwordHash, _ := HashPassword("adminpass")
	adminUser := &User{
		ID:           1,
		Username:     "admin",
		PasswordHash: passwordHash,
		IsAdmin:      true,
	}
	repo.users["admin"] = adminUser
	repo.usersID[1] = adminUser

	// Act: Create another admin user
	user, err := service.CreateUser("alice", "alicepass", true)

	// Assert: Should succeed
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}

	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	if !user.IsAdmin {
		t.Error("expected user to be admin")
	}
}

// TestGetCanonicalAdmin_ReturnsLowestIDAdmin verifies the helper returns the
// first admin in id order — the deterministic "system" user that
// auth-disabled mode resolves every request to.
func TestGetCanonicalAdmin_ReturnsLowestIDAdmin(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	hash, _ := HashPassword("pw")
	// Insert in non-ID order so we know the result wasn't "first inserted".
	repo.users["bob"] = &User{ID: 5, Username: "bob", PasswordHash: hash, IsAdmin: true}
	repo.usersID[5] = repo.users["bob"]
	repo.users["alice"] = &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	repo.usersID[1] = repo.users["alice"]
	repo.users["carol"] = &User{ID: 3, Username: "carol", PasswordHash: hash, IsAdmin: false}
	repo.usersID[3] = repo.users["carol"]

	u, err := service.GetCanonicalAdmin()
	if err != nil {
		t.Fatalf("GetCanonicalAdmin failed: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("got %q, want alice (lowest-id admin)", u.Username)
	}
}

// TestAuthEnabled_DefaultsTrue_WhenNoFnInjected verifies the auth.Service
// reports auth as enabled when no AuthEnabledFn has been wired — i.e. unit
// tests / CLI paths don't need to know about settings.
func TestAuthEnabled_DefaultsTrue_WhenNoFnInjected(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	if !service.AuthEnabled() {
		t.Error("AuthEnabled() = false; want true when no fn injected")
	}
}

// TestAuthEnabled_DelegatesToInjectedFn verifies SetAuthEnabledFn wires the
// settings service in; the Service stays decoupled from the settings package.
func TestAuthEnabled_DelegatesToInjectedFn(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	enabled := true
	service.SetAuthEnabledFn(func() (bool, error) { return enabled, nil })

	if !service.AuthEnabled() {
		t.Error("AuthEnabled() = false; want true when fn returns true")
	}
	enabled = false
	if service.AuthEnabled() {
		t.Error("AuthEnabled() = true; want false when fn returns false")
	}
}

// --- User management (admin actions on other users) ---

// TestListUsers_ReturnsAllUsers verifies the service exposes the full user
// list — feeds the admin Users settings panel.
func TestListUsers_ReturnsAllUsers(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	hash, _ := HashPassword("pw")
	repo.users["alice"] = &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	repo.usersID[1] = repo.users["alice"]
	repo.users["bob"] = &User{ID: 2, Username: "bob", PasswordHash: hash, IsAdmin: false}
	repo.usersID[2] = repo.users["bob"]

	users, err := service.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}
}

// TestDeleteUser_RemovesRow verifies the happy path — a different user is
// deleted and disappears from the user list.
func TestDeleteUser_RemovesRow(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	hash, _ := HashPassword("pw")
	alice := &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	bob := &User{ID: 2, Username: "bob", PasswordHash: hash, IsAdmin: false}
	repo.users["alice"] = alice
	repo.usersID[1] = alice
	repo.users["bob"] = bob
	repo.usersID[2] = bob

	if err := service.DeleteUser(alice.ID, bob.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	if _, exists := repo.users["bob"]; exists {
		t.Error("bob still in repo after delete")
	}
}

// TestDeleteUser_RejectsSelfDelete verifies the actor can't delete their own
// row — protects the admin from accidentally locking themselves out.
func TestDeleteUser_RejectsSelfDelete(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	hash, _ := HashPassword("pw")
	alice := &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	repo.users["alice"] = alice
	repo.usersID[1] = alice

	if err := service.DeleteUser(alice.ID, alice.ID); err == nil {
		t.Error("expected error when actor deletes themselves, got nil")
	}
	if _, exists := repo.users["alice"]; !exists {
		t.Error("alice gone from repo despite self-delete rejection")
	}
}

// TestDeleteUser_RejectsLastAdmin keeps the system from being orphaned by
// deleting its last admin user.
func TestDeleteUser_RejectsLastAdmin(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	hash, _ := HashPassword("pw")
	alice := &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	bob := &User{ID: 2, Username: "bob", PasswordHash: hash, IsAdmin: true}
	repo.users["alice"] = alice
	repo.usersID[1] = alice
	repo.users["bob"] = bob
	repo.usersID[2] = bob

	// alice deletes bob — fine, still one admin left
	if err := service.DeleteUser(alice.ID, bob.ID); err != nil {
		t.Fatalf("DeleteUser(alice→bob) failed: %v", err)
	}

	// Now alice is the sole admin; try to delete her from some hypothetical
	// other admin context (use a different actor ID that doesn't trip the
	// self-delete check).
	if err := service.DeleteUser(99, alice.ID); err == nil {
		t.Error("expected error when deleting last admin, got nil")
	}
	if _, exists := repo.users["alice"]; !exists {
		t.Error("alice removed despite last-admin protection")
	}
}

// TestAdminResetUserPassword_BypassesCurrentPasswordCheck verifies an admin
// can change another user's password without knowing the old one (the
// non-self UpdatePassword path requires the current password).
func TestAdminResetUserPassword_BypassesCurrentPasswordCheck(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())

	oldHash, _ := HashPassword("bobOldPass1234")
	bob := &User{ID: 2, Username: "bob", PasswordHash: oldHash, IsAdmin: false}
	repo.users["bob"] = bob
	repo.usersID[2] = bob

	if err := service.AdminResetUserPassword(bob.ID, "bobNewPass1234"); err != nil {
		t.Fatalf("AdminResetUserPassword failed: %v", err)
	}

	// Old password should no longer authenticate.
	if _, err := service.Authenticate("bob", "bobOldPass1234"); err == nil {
		t.Error("old password still works after reset")
	}
	// New password should authenticate.
	if _, err := service.Authenticate("bob", "bobNewPass1234"); err != nil {
		t.Errorf("new password rejected after reset: %v", err)
	}
}
