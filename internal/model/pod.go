package model

import "time"

// PodRequest contains the inputs needed to construct an ad pod.
//
// Example:
//
//	PodRequest{
//		HouseholdID:      "household-001",
//		DurationSeconds:  120,
//		Candidates:       []Candidate{toyotaCandidate, hondaCandidate},
//		ProtectedCandidate: nil,
//	}
//
// A non-nil ProtectedCandidate represents inventory that must remain in the
// pod. The optimizer may choose among the other candidates marked replaceable.
type PodRequest struct {
	HouseholdID        string
	DurationSeconds    int
	Candidates         []Candidate
	ProtectedCandidate *Candidate
}

// Pod is the selected set of advertisements for one break.
//
// Example:
//
//	Pod{
//		Ads:                   []Candidate{toyotaCandidate, hondaCandidate},
//		DurationSeconds:       60,
//		EstimatedGrossRevenue: 0.0575,
//		Decisions:             []Decision{acceptedToyota, acceptedHonda},
//	}
//
// EstimatedGrossRevenue is calculated from confirmed impressions using CPM /
// 1000. A 120-second pod will eventually contain four 30-second ads.
type Pod struct {
	Ads                   []Candidate
	DurationSeconds       int
	EstimatedGrossRevenue float64
	Decisions             []Decision
}

// Decision explains why a candidate was accepted or rejected.
//
// Example:
//
//	Decision{
//		CandidateID: "amazon-aps-candidate-001",
//		Accepted:    false,
//		Reason:      "duplicate canonical creative already selected",
//	}
type Decision struct {
	CandidateID string
	Accepted    bool
	Reason      string
}

// Reservation represents capacity held while an ad decision is reconciled
// with the eventual impression event.
//
// Example:
//
//	Reservation{
//		ID:          "reservation-001",
//		HouseholdID: "household-001",
//		CreativeID:  "creative-toyota-rav4-spring-30s-001",
//		ExpiresAt:   time.Date(2026, 10, 11, 20, 5, 0, 0, time.UTC),
//	}
//
// A reservation is temporary. It becomes exposure history only after the ad
// is shown and Confirm is called.
type Reservation struct {
	ID          string
	HouseholdID string
	CreativeID  string
	ExpiresAt   time.Time
}

// Impression records the outcome of a selected advertisement.
//
// Example:
//
//	Impression{
//		ReservationID: "reservation-001",
//		HouseholdID:   "household-001",
//		CreativeID:    "creative-toyota-rav4-spring-30s-001",
//		ShownAt:       time.Date(2026, 10, 11, 20, 4, 30, 0, time.UTC),
//	}
type Impression struct {
	ReservationID string
	HouseholdID   string
	CreativeID    string
	ShownAt       time.Time
}
