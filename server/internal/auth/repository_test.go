package auth

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// POST /api/auth/setup is unauthenticated and hashing takes ~100ms, so a
// check-then-insert lets two racing requests both come out with an admin
// account — the second one an attacker's.
func TestCreateFirstAdmin_OnlyOneRequestWins(t *testing.T) {
	db := setupSessionTestDB(t)
	repo := NewRepository(db).(*repository)

	const racers = 8
	var wg sync.WaitGroup
	errs := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = repo.CreateFirstAdmin(fmt.Sprintf("admin%d", i), "hash")
		}(i)
	}
	wg.Wait()

	won := 0
	for i, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrSetupAlreadyCompleted):
		default:
			t.Errorf("racer %d got an unexpected error: %v", i, err)
		}
	}
	if won != 1 {
		t.Errorf("%d racers completed setup, want exactly 1", won)
	}

	count, err := repo.GetUserCount()
	if err != nil {
		t.Fatalf("GetUserCount: %v", err)
	}
	if count != 1 {
		t.Errorf("user table holds %d rows after a contested setup, want 1", count)
	}
}

func TestCreateFirstAdmin_RefusesOnceAUserExists(t *testing.T) {
	db := setupSessionTestDB(t)
	repo := NewRepository(db).(*repository)

	if _, err := repo.CreateFirstAdmin("owner", "hash"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if _, err := repo.CreateFirstAdmin("mallory", "hash"); !errors.Is(err, ErrSetupAlreadyCompleted) {
		t.Errorf("got %v, want ErrSetupAlreadyCompleted", err)
	}
}

// The driver's constraint text must be mapped in the repository, never
// bubbled up for a handler to echo at a client.
func TestCreate_DuplicateUsernameMapsToDomainError(t *testing.T) {
	db := setupSessionTestDB(t)
	repo := NewRepository(db)

	if _, err := repo.Create("alice", "hash", false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := repo.Create("alice", "hash", false); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("got %v, want ErrUsernameTaken", err)
	}
}
