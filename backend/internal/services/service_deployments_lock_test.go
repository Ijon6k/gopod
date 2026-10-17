package services

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestDeployLock_ConcurrentStartsAreSerialized verifies the per-service deploy
// mutex guarantees mutual exclusion under concurrency: exactly one holder is
// active at any moment, and every worker eventually gets the lock.
func TestDeployLock_ConcurrentStartsAreSerialized(t *testing.T) {
	ws := &WorkloadService{}
	const workers = 16

	var mu sync.Mutex
	current, successes, violations := 0, 0, 0
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			depID := fmt.Sprintf("dep-%d", i)
			for attempt := 0; ; attempt++ {
				if holder := ws.tryAcquireDeploy("svc-lock-test", depID); holder != "" {
					if attempt > 1000 {
						return
					}
					time.Sleep(time.Millisecond)
					continue
				}
				break
			}

			mu.Lock()
			successes++
			current++
			if current > 1 {
				violations++
			}
			mu.Unlock()

			time.Sleep(5 * time.Millisecond) // simulate deploy work

			mu.Lock()
			current--
			mu.Unlock()
			ws.releaseDeploy("svc-lock-test", depID)
		}(i)
	}
	wg.Wait()

	if violations != 0 {
		t.Fatalf("mutual exclusion violated %d time(s): multiple holders ran concurrently", violations)
	}
	if successes != workers {
		t.Fatalf("expected all %d workers to eventually acquire the lock, got %d", workers, successes)
	}
}

// TestDeployLock_ReleaseOnlyByOwner verifies a foreign release can never clear
// someone else's lock and that the owner's release frees the service again.
func TestDeployLock_ReleaseOnlyByOwner(t *testing.T) {
	ws := &WorkloadService{}

	if holder := ws.tryAcquireDeploy("svc-owner-test", "dep-a"); holder != "" {
		t.Fatalf("first acquire must succeed, got holder %q", holder)
	}
	if holder := ws.tryAcquireDeploy("svc-owner-test", "dep-b"); holder != "dep-a" {
		t.Fatalf("second acquire must conflict with current holder, got holder %q", holder)
	}
	ws.releaseDeploy("svc-owner-test", "dep-b") // wrong owner: must be a no-op
	if holder := ws.tryAcquireDeploy("svc-owner-test", "dep-c"); holder != "dep-a" {
		t.Fatalf("lock must still be held by dep-a after a foreign release, got holder %q", holder)
	}
	ws.releaseDeploy("svc-owner-test", "dep-a")
	if holder := ws.tryAcquireDeploy("svc-owner-test", "dep-c"); holder != "" {
		t.Fatalf("lock must be acquirable after owner release, got holder %q", holder)
	}
}
