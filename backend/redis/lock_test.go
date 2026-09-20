package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	redisPkg "github.com/wikicollab/backend/redis"
)

// testLockStore opens a LockStore against the test Redis instance.
// Skips if TEST_REDIS_URL is not set.
func testLockStore(t *testing.T) *redisPkg.LockStore {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = os.Getenv("REDIS_URL")
	}
	if url == "" {
		t.Skip("TEST_REDIS_URL not set — skipping Redis integration test")
	}

	client, err := redisPkg.NewClient(url)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// Flush test keys to ensure a clean state.
	ctx := context.Background()
	client.FlushDB(ctx)

	return redisPkg.NewLockStore(client)
}

const (
	testPageID  = "page-test-001"
	testBlockID = "block-test-001"
)

func TestAcquireLock_Success(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	acquired, err := store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}
}

func TestAcquireLock_AlreadyHeld(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	_, err := store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	// Second acquire from a different token should fail.
	acquired, err := store.AcquireLock(ctx, testPageID, testBlockID, "token-bob", 30*time.Second)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if acquired {
		t.Fatal("expected second acquire to be denied")
	}
}

func TestAcquireLock_SameToken_Fails(t *testing.T) {
	// SET NX will reject even the same token — the client should renew instead.
	store := testLockStore(t)
	ctx := context.Background()

	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)

	acquired, err := store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	if acquired {
		t.Fatal("expected NX to prevent duplicate SET")
	}
}

func TestRenewLock_CorrectToken(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)

	renewed, err := store.RenewLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !renewed {
		t.Fatal("expected renew to succeed with correct token")
	}
}

func TestRenewLock_WrongToken(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)

	renewed, err := store.RenewLock(ctx, testPageID, testBlockID, "token-impostor", 30*time.Second)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if renewed {
		t.Fatal("expected renew to fail with wrong token")
	}
}

func TestReleaseLock_CorrectToken(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)

	released, err := store.ReleaseLock(ctx, testPageID, testBlockID, "token-alice")
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if !released {
		t.Fatal("expected release to succeed with correct token")
	}

	// Lock should be gone — another user can acquire it now.
	acquired, _ := store.AcquireLock(ctx, testPageID, testBlockID, "token-bob", 30*time.Second)
	if !acquired {
		t.Fatal("expected new acquire to succeed after release")
	}
}

func TestReleaseLock_WrongToken(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 30*time.Second)

	released, err := store.ReleaseLock(ctx, testPageID, testBlockID, "token-impostor")
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if released {
		t.Fatal("expected release to fail with wrong token")
	}

	// Original lock must still be held by alice.
	owner, found, err := store.GetLockOwner(ctx, testPageID, testBlockID)
	if err != nil {
		t.Fatalf("get lock owner: %v", err)
	}
	if !found {
		t.Fatal("expected lock to still exist after failed release")
	}
	if owner != "token-alice" {
		t.Errorf("expected owner=token-alice, got %s", owner)
	}
}

func TestLock_Expiry(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	// Acquire with a very short TTL.
	store.AcquireLock(ctx, testPageID, testBlockID, "token-alice", 200*time.Millisecond)

	// Wait for it to expire.
	time.Sleep(400 * time.Millisecond)

	// Lock should be gone — another user can acquire it.
	acquired, err := store.AcquireLock(ctx, testPageID, testBlockID, "token-bob", 30*time.Second)
	if err != nil {
		t.Fatalf("acquire after expiry: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquirable after TTL expiry")
	}
}

func TestGetLockOwner_NoLock(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	_, found, err := store.GetLockOwner(ctx, testPageID, "nonexistent-block")
	if err != nil {
		t.Fatalf("get owner: %v", err)
	}
	if found {
		t.Fatal("expected no lock found for nonexistent block")
	}
}

func TestReleaseLock_NonExistent(t *testing.T) {
	store := testLockStore(t)
	ctx := context.Background()

	// Releasing a lock that doesn't exist should not error.
	released, err := store.ReleaseLock(ctx, testPageID, "ghost-block", "any-token")
	if err != nil {
		t.Fatalf("release nonexistent: %v", err)
	}
	if released {
		t.Fatal("expected released=false for nonexistent lock")
	}
}
