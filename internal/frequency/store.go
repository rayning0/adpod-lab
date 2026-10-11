// Package frequency contains household exposure state implementations.
package frequency

import (
	"context"
	"time"
)

// FrequencyStore coordinates reservation and impression state for one household.
//
// The first implementation is intentionally process-local. A later shared
// implementation can satisfy this interface without changing the optimizer.
type FrequencyStore interface {
	Reserve(ctx context.Context, householdID, creativeID string, limit int, window, ttl time.Duration) (reservationID string, ok bool, err error)
	Confirm(ctx context.Context, reservationID string, at time.Time) error
	Cancel(ctx context.Context, reservationID string) error
}
