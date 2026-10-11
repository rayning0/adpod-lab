package model

import "time"

// PodRequest contains the inputs needed to construct an ad pod.
type PodRequest struct {
	HouseholdID        string
	DurationSeconds    int
	Candidates         []Candidate
	ProtectedCandidate *Candidate
}

// Pod is the selected set of advertisements for one break.
type Pod struct {
	Ads                   []Candidate
	DurationSeconds       int
	EstimatedGrossRevenue float64
	Decisions             []Decision
}

// Decision explains why a candidate was accepted or rejected.
type Decision struct {
	CandidateID string
	Accepted    bool
	Reason      string
}

// Reservation represents capacity held while an ad decision is reconciled
// with the eventual impression event.
type Reservation struct {
	ID          string
	HouseholdID string
	CreativeID  string
	ExpiresAt   time.Time
}

// Impression records the outcome of a selected advertisement.
type Impression struct {
	ReservationID string
	HouseholdID   string
	CreativeID    string
	ShownAt       time.Time
}
