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
