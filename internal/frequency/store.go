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
//
// Example lifecycle:
//
//	reservationID, ok, err := store.Reserve(
//		ctx, "household-001", "creative-toyota-rav4-spring-30s-001",
//		3, time.Hour, 2*time.Minute,
//	)
//	// If the ad is shown: store.Confirm(ctx, reservationID, shownAt)
//	// If the ad is not shown: store.Cancel(ctx, reservationID)
type FrequencyStore interface {
	Reserve(ctx context.Context, householdID, creativeID string, limit int, window, ttl time.Duration) (reservationID string, ok bool, err error)
	Confirm(ctx context.Context, reservationID string, at time.Time) error
	Cancel(ctx context.Context, reservationID string) error
}
