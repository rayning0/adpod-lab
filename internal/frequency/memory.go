package frequency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrCapped             = errors.New("frequency cap reached")
	ErrUnknownReservation = errors.New("unknown reservation")
	ErrCanceled           = errors.New("reservation canceled")
	ErrConfirmed          = errors.New("reservation already confirmed")
)

// Identifies frequency bucket shared by 1 household and 1 canonical creative.
type Key struct {
	HouseholdID string
	CreativeID  string
}

// STATE MACHINE for "reservation":

// active ──Confirm──> confirmed
// active ──Cancel───> canceled

// confirmed ──Confirm again──> confirmed  (successful, idempotent)
// canceled  ──Cancel again───> canceled   (successful, idempotent)

// confirmed ──Cancel───> error: ErrConfirmed
// canceled  ──Confirm──> error: ErrCanceled
// expired (TTL) active ───────> deleted / ErrUnknownReservation

type reservationStatus uint8 // enum

const (
	statusActive    reservationStatus = iota // 0
	statusConfirmed                          // 1
	statusCanceled                           // 2
)

type reservation struct {
	key         Key
	status      reservationStatus
	expiresAt   time.Time
	window      time.Duration
	confirmedAt time.Time
}

// InMemoryFrequencyStore is a process-local, concurrency-safe frequency store.
// Its mutex makes reservation decisions atomic within this process only.
type InMemoryFrequencyStore struct {
	mu sync.Mutex

	defaultLimit  int
	defaultWindow time.Duration
	now           func() time.Time
	nextID        uint64

	impressions  map[Key][]time.Time
	reservations map[string]*reservation
}

// NewInMemoryFrequencyStore creates a store with defaults used when callers
// provide non-positive limit or window values to Reserve.
func NewInMemoryFrequencyStore(limit int, window time.Duration, now func() time.Time) (*InMemoryFrequencyStore, error) {
	if limit <= 0 {
		return nil, errors.New("frequency limit must be positive")
	}
	if window <= 0 {
		return nil, errors.New("frequency window must be positive")
	}
	if now == nil {
		now = time.Now
	}
	return &InMemoryFrequencyStore{
		defaultLimit:  limit,
		defaultWindow: window,
		now:           now,
		impressions:   make(map[Key][]time.Time),
		reservations:  make(map[string]*reservation),
	}, nil
}

// Atomically checks confirmed impressions and active reservations, then reserves 1 slot.
// r.status = statusActive
// s.reservations[id] = pointer to reservation struct

// Returns (string, bool, error):
// - reservation ID
// - ok: if capacity was available and reserved
// - error: bad input or canceled context
// Normal frequency-cap denial returns ("", false, nil), not an error.

// (s *InMemoryFrequencyStore) is a pointer receiver.
// It means Reserve() gets pointer to the real store, not a copy. It must do so because it
// changes s.nextID, s.impressions, and s.reservations; it also must lock the 1 shared mutex.
func (s *InMemoryFrequencyStore) Reserve(ctx context.Context, householdID, creativeID string, limit int, window, ttl time.Duration) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if householdID == "" || creativeID == "" {
		return "", false, errors.New("household and creative IDs are required")
	}
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if window <= 0 {
		window = s.defaultWindow
	}
	if limit <= 0 || window <= 0 {
		return "", false, errors.New("frequency limit and window must be positive")
	}
	if ttl <= 0 {
		return "", false, errors.New("reservation TTL must be positive")
	}

	// Lock() waits till this goroutine exclusively owns the mutex.
	// "defer" schedules Unlock() to run when Reserve() returns---whether it succeeds, finds the cap reached, or hits later error. This way, store won't be permanently locked.
	// While this lock is held, no other Reserve(), Confirm(), or Cancel() call can inspect/change the shared state.
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	key := Key{HouseholdID: householdID, CreativeID: creativeID}
	s.expireLocked(key, now, window)

	confirmed := len(s.impressions[key])
	active := 0
	for _, r := range s.reservations {
		if r.key == key && r.status == statusActive && r.expiresAt.After(now) {
			active++
		}
	}

	// If # of confirmed impressions + active reservations > freq cap, reservation fails. It's NOT error!
	if confirmed+active >= limit {
		return "", false, nil
	}

	s.nextID++
	id := fmt.Sprintf("reservation-%d", s.nextID)
	s.reservations[id] = &reservation{
		key:       key,
		status:    statusActive,
		expiresAt: now.Add(ttl),
		window:    window,
	}
	return id, true, nil
}

// Converts an active reservation into exactly 1 impression.
// r.status = statusConfirmed
// s.impressions[r.key] = append(s.impressions[r.key], view time)
func (s *InMemoryFrequencyStore) Confirm(ctx context.Context, id string, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.reservations[id]
	if !ok {
		return ErrUnknownReservation
	}

	// only reservations with "statusActive" get by this
	switch r.status {
	case statusConfirmed:
		return nil

	// May not confirm a canceled reservation
	case statusCanceled:
		return ErrCanceled
	}

	now := s.now()
	if at.IsZero() {
		at = now
	}

	// delete expired reservation
	if !r.expiresAt.After(now) {
		delete(s.reservations, id)
		return ErrUnknownReservation
	}
	r.status = statusConfirmed
	r.confirmedAt = at
	s.expireLocked(r.key, at, r.window)
	s.impressions[r.key] = append(s.impressions[r.key], at)
	return nil
}

// Cancel an active reservation. Repeated cancellation is idempotent.
// r.status = statusCanceled
func (s *InMemoryFrequencyStore) Cancel(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.reservations[id]
	if !ok {
		return ErrUnknownReservation
	}
	switch r.status {
	case statusCanceled:
		return nil

	// May not cancel a confirmed reservation
	case statusConfirmed:
		return ErrConfirmed
	}

	if !r.expiresAt.After(s.now()) {
		delete(s.reservations, id)
		return ErrUnknownReservation
	}
	r.status = statusCanceled
	return nil

}

// expireLocked() deletes expired reservations and impressions for key.
// The caller must hold s.mu.
func (s *InMemoryFrequencyStore) expireLocked(key Key, now time.Time, window time.Duration) {
	for id, r := range s.reservations {
		if r.key == key && r.status == statusActive && !r.expiresAt.After(now) {
			delete(s.reservations, id)
		}
	}

	cutoff := now.Add(-window)
	existing := s.impressions[key]
	kept := existing[:0]
	for _, timestamp := range existing {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	if len(kept) == 0 {
		delete(s.impressions, key)
	} else {
		s.impressions[key] = kept
	}
}

var _ FrequencyStore = (*InMemoryFrequencyStore)(nil)
