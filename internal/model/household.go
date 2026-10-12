package model

// Household identifies shared viewing context across devices and
// channels. It is intentionally small for the first local simulation.
//
// Example:
//
//	Household{
//		ID:      "household-001",
//		Devices: []string{"living-room-roku-001", "bedroom-fire-tv-001"},
//	}
//
// Both devices contribute to the same household frequency history. This is
// why a Toyota ad viewed on one device can affect eligibility on the other.
type Household struct {
	ID      string   `json:"household_id"`
	Devices []string `json:"devices"`
}
