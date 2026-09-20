package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// lockKey builds the Redis key for a block lock.
// Format: lock:page:{pageID}:block:{blockID}
func lockKey(pageID, blockID string) string {
	return fmt.Sprintf("lock:page:%s:block:%s", pageID, blockID)
}

// LockStore handles all block-lock operations against Redis.
type LockStore struct {
	client *goredis.Client
}

// NewLockStore creates a LockStore.
func NewLockStore(client *goredis.Client) *LockStore {
	return &LockStore{client: client}
}

// AcquireLock attempts to atomically set the lock key to token with a TTL.
// Uses SET NX EX — safe against race conditions.
// Returns true if the lock was acquired, false if it is already held.
func (s *LockStore) AcquireLock(ctx context.Context, pageID, blockID, token string, ttl time.Duration) (bool, error) {
	key := lockKey(pageID, blockID)
	ok, err := s.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock %s: %w", key, err)
	}
	return ok, nil
}

// RenewLock extends the TTL of the lock only if the caller still owns it (token matches).
// Returns true if the renewal succeeded, false if the lock is not owned by this token.
func (s *LockStore) RenewLock(ctx context.Context, pageID, blockID, token string, ttl time.Duration) (bool, error) {
	key := lockKey(pageID, blockID)

	// Lua: only EXPIRE if the current value equals the caller's token.
	// This is atomic: no window between GET and EXPIRE.
	const renewScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	ttlMs := ttl.Milliseconds()
	result, err := s.client.Eval(ctx, renewScript, []string{key}, token, ttlMs).Int()
	if err != nil {
		return false, fmt.Errorf("renew lock %s: %w", key, err)
	}
	return result == 1, nil
}

// ReleaseLock deletes the lock key only if the caller owns it (token matches).
// Uses a Lua script for atomicity — prevents a delayed release from evicting a lock
// that has since been acquired by a different user.
// Returns true if the lock was released, false if it was not owned by this token.
func (s *LockStore) ReleaseLock(ctx context.Context, pageID, blockID, token string) (bool, error) {
	key := lockKey(pageID, blockID)

	// Lua: GET then DEL only when token matches — atomic.
	const releaseScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
	result, err := s.client.Eval(ctx, releaseScript, []string{key}, token).Int()
	if err != nil {
		return false, fmt.Errorf("release lock %s: %w", key, err)
	}
	return result == 1, nil
}

// GetLockOwner returns the token currently stored for the given block lock.
// Returns ("", false) if no lock is held.
func (s *LockStore) GetLockOwner(ctx context.Context, pageID, blockID string) (string, bool, error) {
	key := lockKey(pageID, blockID)
	val, err := s.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get lock %s: %w", key, err)
	}
	return val, true, nil
}
