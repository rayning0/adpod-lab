package frequency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Make frequency store with freq cap = limit, window = 1 hour, start time = now
func testStore(t *testing.T, limit int, now func() time.Time) *InMemoryFrequencyStore {
	t.Helper()
	store, err := NewInMemoryFrequencyStore(limit, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// Reserve 1 ad impression slot for "household ID + creative ID".
// Use default freq cap and default window. TTL = expiration time = 1 minute.
func reserve(t *testing.T, store *InMemoryFrequencyStore, household, creative string) string {
	t.Helper()
	id, ok, err := store.Reserve(context.Background(), household, creative, 0, 0, time.Minute)
	if err != nil || !ok {
		t.Fatalf("reserve = %q, %t, %v", id, ok, err)
	}
	return id
}

// Confirm an active reservation as 1 viewed impression at specific time.
func confirm(t *testing.T, store *InMemoryFrequencyStore, id string) {
	t.Helper()
	if err := store.Confirm(context.Background(), id, time.Time{}); err != nil {
		t.Fatal(err)
	}
}

// Test main state transitions:

// 1. Confirm path
// Creates a freq cap = 1 store.
// Reserves a household/creative slot.
// Confirms it.
// Confirms it again. Idempotent OK.
// Verifies duplicate confirmation is harmless.

// Tries 2nd reservation. Confirms it's rejected due to freq cap.
// Verifies the confirmed impression consumes the only slot.

// Tries to cancel confirmed reservation. Verifies it FAILS with ErrConfirmed.

// 2. Cancel path
// Reserves a different household/creative key.
// Cancels it.
// Cancels it again. Idempotent OK.
// Verifies repeated cancellation is harmless.

// Tries to confirm canceled reservation. Verifies it FAILS with ErrCanceled.

// Reserves different household/creative key again, successfully. Verifies cancellation released freq capacity.

// Trying to confirm reservation with no ID should FAIL.
func TestReserveConfirmAndCancelLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	store := testStore(t, 1, func() time.Time { return now })

	// 1. Confirm path

	id := reserve(t, store, "household-1", "creative-1")
	if err := store.Confirm(context.Background(), id, now); err != nil {
		t.Fatal(err)
	}
	// 2nd confirm is OK. Idempotent.
	if err := store.Confirm(context.Background(), id, now); err != nil {
		t.Fatalf("Duplicate confirm: %v", err)
	}

	// This 2nd reservation is REJECTED for hitting freq cap.
	// A freq cap rejection returns (id = "", ok = false, err = nil)
	if _, ok, err := store.Reserve(context.Background(), "household-1", "creative-1", 1, time.Hour, time.Minute); err != nil || ok {
		t.Fatalf("Confirmed impression did not consume capacity: ok=%t err=%v", ok, err)
	}

	// Canceling confirmed reservation should FAIL
	if err := store.Cancel(context.Background(), id); !errors.Is(err, ErrConfirmed) {
		t.Fatalf("Canceling confirmed reservation succeeded. Wrong. Got %v. Should get ErrConfirmed.", err)
	}

	// 2. Cancel path

	id2 := reserve(t, store, "household-2", "creative-1")
	if err := store.Cancel(context.Background(), id2); err != nil {
		t.Fatal(err)
	}
	// 2nd cancel is OK. Idempotent.
	if err := store.Cancel(context.Background(), id2); err != nil {
		t.Fatalf("Duplicate cancel: %v", err)
	}

	// Confirming canceled reservation should FAIL
	if err := store.Confirm(context.Background(), id2, now); !errors.Is(err, ErrCanceled) {
		t.Fatalf("Confirming canceled reservation succeeded. Wrong. Got %v. Should get ErrCanceled.", err)
	}

	// Reserves "household-2", "creative-1" again, sucessfully.
	// Shows cancellation released freq capacity.
	if _, ok, err := store.Reserve(context.Background(), "household-2", "creative-1", 1, time.Hour, time.Minute); err != nil || !ok {
		t.Fatalf("Canceled reservation didn't release freq capacity: ok=%t err=%v", ok, err)
	}

	// Trying to confirm reservation with no ID should FAIL.
	if err := store.Confirm(context.Background(), "no id", now); !errors.Is(err, ErrUnknownReservation) {
		t.Fatalf("confirm unknown = %v, want ErrUnknownReservation", err)
	}
}

// Test rolling window boundary.

// Timeline:
// 00:00  Reserve ad and Confirm 1 impression.
// 00:30  Still frequency capped
// 00:59  Still frequency capped
// 01:00  Original impression expires. Can now successfully reserve new ad.

// Checks an impression exactly 1 hour old no longer counts.
func TestRollingWindowExpiresAtBoundary(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	store := testStore(t, 1, func() time.Time { return now })
	confirm(t, store, reserve(t, store, "household-1", "creative-1"))

	for _, advance := range []time.Duration{30 * time.Minute, 59 * time.Minute} {
		now = time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC).Add(advance)
		if _, ok, err := store.Reserve(context.Background(), "household-1", "creative-1", 1, time.Hour, time.Minute); err != nil || ok {
			t.Fatalf("reserve at %s: ok=%t err=%v, want capped", now, ok, err)
		}
	}
	now = time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	if _, ok, err := store.Reserve(context.Background(), "household-1", "creative-1", 1, time.Hour, time.Minute); err != nil || !ok {
		t.Fatalf("reserve at boundary: ok=%t err=%v, want success", ok, err)
	}
}

// Test we can successfully make 3 different reservations with 3 different keys:
// household-1 + creative-1
// household-2 + creative-1
// household-1 + creative-2
// None of these keys should conflict with each other.
func TestHouseholdAndCreativeIsolation(t *testing.T) {
	now := time.Now
	store := testStore(t, 1, now)
	confirm(t, store, reserve(t, store, "household-1", "creative-1"))

	if _, ok, _ := store.Reserve(context.Background(), "household-2", "creative-1", 1, time.Hour, time.Minute); !ok {
		t.Fatal("different household was capped")
	}
	if _, ok, _ := store.Reserve(context.Background(), "household-1", "creative-2", 1, time.Hour, time.Minute); !ok {
		t.Fatal("different creative was capped")
	}
}

// Test expired reservations.
// Steps:

// Reserves a freq cap = 1 slot.
// Advances time by 1 minute, exactly to reservation TTL. Reservation should expire.
// Attempts another reservation. It should succeed.
// Checks expired reservation no longer consumes capacity.
// This protects against an abandoned selection permanently blocking a slot.
func TestExpiredReservationReleasesCapacity(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	store := testStore(t, 1, func() time.Time { return now })
	reserve(t, store, "household-1", "creative-1")
	now = now.Add(time.Minute)
	if _, ok, err := store.Reserve(context.Background(), "household-1", "creative-1", 1, time.Hour, time.Minute); err != nil || !ok {
		t.Fatalf("expired reservation did not release capacity: ok=%t err=%v", ok, err)
	}
}

// Concurrency test.

// Cap:                   3
// Confirmed impressions: 2
// Remaining capacity:    1
// Concurrent requests: 100
// Steps:

// Creates 2 confirmed impressions.
// Starts 100 goroutines.
// Each goroutine attempts to reserve the same household/creative key.
// All calls use the same freq cap and window.
// The mutex serializes the check-and-create operation.
// The test counts successful reservations using atomic.Int32.
// It expects only ONE successful new reservation!
func TestConcurrentReserveDoesNotExceedCap(t *testing.T) {
	store := testStore(t, 3, time.Now)
	for i := 0; i < 2; i++ {
		confirm(t, store, reserve(t, store, "household-1", "creative-1"))
	}

	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.Reserve(context.Background(), "household-1", "creative-1", 3, time.Hour, time.Minute)
			if err != nil {
				t.Errorf("reserve error: %v", err)
			} else if ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("successful reservations = %d, want 1", got)
	}
}
