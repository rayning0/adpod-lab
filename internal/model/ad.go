package model

// Candidate is one offer for an advertisement from a demand source.
//
// Example:
//
//	Candidate{
//		CandidateID:        "amazon-aps-candidate-001",
//		Source:             "amazon-aps",
//		SourceCreativeID:   "aps-creative-8472",
//		UniversalAdID:      "toyota-rav4-spring-30s-001",
//		MediaKey:           "sha256:toyota-rav4-30s-1080p-v1-001",
//		FixtureFingerprint: "fp-toyota-rav4-30s-001",
//		AdvertiserID:       "toyota",
//		ProductID:          "rav4",
//		DurationSeconds:    30,
//		CPM:                28.50,
//		InventoryOwner:     "simulated",
//		Replaceable:        true,
//	}
//
// Source is the demand source or ad-system partner that supplied the offer;
// it is not the advertiser. AdvertiserID identifies Toyota in this example.
// SourceCreativeID is the identifier assigned by that source to the ad. It
// may be different from the identifier assigned by another source.
//
// MediaKey identifies one exact media asset, usually with a cryptographic
// file hash. Two byte-for-byte identical files have the same MediaKey, while
// two differently transcoded files normally have different MediaKeys.
//
// FixtureFingerprint is an opaque, precomputed identity signal used by this
// project. It can represent a simulated audio fingerprint, video fingerprint,
// or combined audio/video fingerprint. The MVP does not extract it from media.

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
//
// Example:
//
//	ResolvedCreative{
//		CandidateID:         "amazon-aps-candidate-001",
//		CanonicalCreativeID: "creative-toyota-rav4-spring-30s-001",
//		Method:              "fixture_fingerprint",
//		Confidence:          0.96,
//	}
//
// CandidateID identifies one source offer. CanonicalCreativeID identifies the
// underlying commercial, allowing different candidates for the same ad to be
// deduplicated across sources and media variants.
type ResolvedCreative struct {
	CandidateID         string  `json:"candidate_id"`
	CanonicalCreativeID string  `json:"canonical_creative_id,omitempty"`
	Method              string  `json:"resolution_method"`
	Confidence          float64 `json:"confidence"`
}
