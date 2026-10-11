package model

import (
	"strings"
	"testing"
)

func TestLoadCandidates(t *testing.T) {
	input := `[
		{
			"candidate_id": "source-a-001",
			"source": "source-a",
			"source_creative_id": "a-42",
			"advertiser_id": "autonova",
			"product_id": "sedan-x",
			"duration_seconds": 30,
			"cpm": 28.5,
			"inventory_owner": "simulated",
			"replaceable": true
		}
	]`

	candidates, err := LoadCandidates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("LoadCandidates() returned %d candidates, want 1", len(candidates))
	}
	if candidates[0].CandidateID != "source-a-001" {
		t.Errorf("CandidateID = %q, want %q", candidates[0].CandidateID, "source-a-001")
	}
	if candidates[0].CPM != 28.5 {
		t.Errorf("CPM = %v, want 28.5", candidates[0].CPM)
	}
}
