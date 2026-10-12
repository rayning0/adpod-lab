package model

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadCandidates decodes a JSON array of candidates from r.
func LoadCandidates(r io.Reader) ([]Candidate, error) {
	var candidates []Candidate
	if err := json.NewDecoder(r).Decode(&candidates); err != nil {
		return nil, fmt.Errorf("decode candidates: %w", err)
	}
	return candidates, nil
}

// LoadCandidatesFile decodes a JSON candidate fixture from path.
func LoadCandidatesFile(path string) ([]Candidate, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open candidates fixture: %w", err)
	}
	defer file.Close()

	return LoadCandidates(file)
}

// Creative is a canonical commercial definition, independent of any source
// candidate or media encoding.
type Creative struct {
	CreativeID      string `json:"creative_id"`
	Title           string `json:"title"`
	AdvertiserID    string `json:"advertiser_id"`
	AdvertiserName  string `json:"advertiser_name"`
	ProductID       string `json:"product_id"`
	ProductName     string `json:"product_name"`
	DurationSeconds int    `json:"duration_seconds"`
}

// LoadCreatives decodes a JSON array of canonical creatives from r.
func LoadCreatives(r io.Reader) ([]Creative, error) {
	var creatives []Creative
	if err := json.NewDecoder(r).Decode(&creatives); err != nil {
		return nil, fmt.Errorf("decode creatives: %w", err)
	}
	return creatives, nil
}

// LoadCreativesFile decodes canonical creatives from path.
func LoadCreativesFile(path string) ([]Creative, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open creatives fixture: %w", err)
	}
	defer file.Close()
	return LoadCreatives(file)
}

// LoadGroundTruth decodes the test/evaluation-only candidate mapping from r.
func LoadGroundTruth(r io.Reader) (map[string]string, error) {
	var groundTruth map[string]string
	if err := json.NewDecoder(r).Decode(&groundTruth); err != nil {
		return nil, fmt.Errorf("decode ground truth: %w", err)
	}
	return groundTruth, nil
}

// LoadGroundTruthFile decodes ground truth from path.
func LoadGroundTruthFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ground truth fixture: %w", err)
	}
	defer file.Close()
	return LoadGroundTruth(file)
}
