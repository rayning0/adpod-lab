package model

// Candidate is an advertisement offered by a simulated demand source.
type Candidate struct {
	CandidateID        string  `json:"candidate_id"`
	Source             string  `json:"source"`
	SourceCreativeID   string  `json:"source_creative_id"`
	UniversalAdID      string  `json:"universal_ad_id,omitempty"`
	MediaKey           string  `json:"media_key,omitempty"`
	FixtureFingerprint string  `json:"fixture_fingerprint,omitempty"`
	AdvertiserID       string  `json:"advertiser_id"`
	ProductID          string  `json:"product_id"`
	DurationSeconds    int     `json:"duration_seconds"`
	CPM                float64 `json:"cpm"`
	InventoryOwner     string  `json:"inventory_owner"`
	Replaceable        bool    `json:"replaceable"`
}

// ResolvedCreative records the canonical identity inferred for a candidate.
type ResolvedCreative struct {
	CandidateID         string  `json:"candidate_id"`
	CanonicalCreativeID string  `json:"canonical_creative_id,omitempty"`
	Method              string  `json:"resolution_method"`
	Confidence          float64 `json:"confidence"`
}
