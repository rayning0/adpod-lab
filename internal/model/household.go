package model

// Household identifies the shared viewing context across devices and
// channels. It is intentionally small for the first local simulation.
type Household struct {
	ID      string   `json:"household_id"`
	Devices []string `json:"devices"`
}
