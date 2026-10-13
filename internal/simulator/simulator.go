// Package simulator contains the deterministic viewing replay used by the CLI.
package simulator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/rgan/adpod-lab/internal/creative"
	"github.com/rgan/adpod-lab/internal/fixtures"
	"github.com/rgan/adpod-lab/internal/frequency"
	"github.com/rgan/adpod-lab/internal/model"
	"github.com/rgan/adpod-lab/internal/pod"
)

const (
	DefaultSeed        int64 = 20261011
	DefaultHouseholds        = 20
	DefaultDevices           = 3
	DefaultChannels          = 5
	DefaultMinutes           = 120
	FrequencyLimit           = 3
	FrequencyWindow          = time.Hour
	ReservationTTL           = 5 * time.Minute
	PodDurationSeconds       = 120
	SlotsPerPod              = 4
)

// EventType is an event in the immutable simulated timeline.
type EventType string

const (
	EventViewingStart    EventType = "viewing_start"
	EventChannelSwitch   EventType = "channel_switch"
	EventAdBreak         EventType = "ad_break"
	EventPlaybackCancel  EventType = "playback_cancel"
	EventPlaybackConfirm EventType = "playback_confirm"
	EventPlaybackDelayed EventType = "playback_delayed"
)

// Outcome is generated before either strategy is replayed.
type Outcome string

const (
	OutcomeConfirmed Outcome = "confirmed"
	OutcomeCanceled  Outcome = "canceled"
	OutcomeDelayed   Outcome = "delayed"
)

// ViewingEvent contains enough information to replay one viewing opportunity.
// Outcome is populated only for playback events. OpportunityID and Slot
// associate a playback result with a pod slot, never with a candidate ID.
type ViewingEvent struct {
	EventID       string
	Timestamp     time.Time
	HouseholdID   string
	DeviceID      string
	ChannelID     string
	EventType     EventType
	OpportunityID string
	Slot          int
	Outcome       Outcome
}

// Config controls event generation and replay. Candidates and registry are
// supplied by the existing fixture loaders; when omitted, deterministic
// generated fixtures are used for package-level tests.
type Config struct {
	Seed        int64
	Households  int
	MaxDevices  int
	Channels    int
	Minutes     int
	Candidates  []model.Candidate
	Registry    model.IdentityRegistry
	GroundTruth model.ResolutionExpectations
}

// DefaultConfig returns the CLI's development configuration.
func DefaultConfig() Config {
	return Config{Seed: DefaultSeed, Households: DefaultHouseholds, MaxDevices: DefaultDevices, Channels: DefaultChannels, Minutes: DefaultMinutes}
}

// Metrics is the evaluation report for one strategy.
type Metrics struct {
	ProcessedBreaks      int
	AdOpportunities      int
	SelectedAds          int
	ConfirmedImpressions int
	CanceledImpressions  int
	DelayedConfirmations int
	// This combines late/expired and unknown callback failures because the
	// FrequencyStore interface exposes one unknown-reservation error for both.
	ExpiredOrUnknownSettlements    int
	IncompletePods                 int
	IntraPodDuplicates             int
	AvoidableIntraPodDuplicates    int
	UnavoidableProtectedDuplicates int
	FrequencyCapViolations         int
	EstimatedRevenue               float64
	ConfirmedRevenue               float64
	FillRate                       float64
	UnresolvedIdentities           int
}

// StrategyReport names the strategy and its measured metrics.
type StrategyReport struct {
	Name    string
	Metrics Metrics
}

// Report contains the shared event stream and isolated replay reports.
type Report struct {
	Seed      int64
	Events    []ViewingEvent
	Baseline  StrategyReport
	Optimized StrategyReport
}

// GenerateEvents makes one immutable, seed-controlled timeline.
func GenerateEvents(cfg Config) []ViewingEvent {
	if cfg.Seed == 0 {
		cfg.Seed = DefaultSeed
	}
	if cfg.Households <= 0 {
		cfg.Households = DefaultHouseholds
	}
	if cfg.MaxDevices <= 0 {
		cfg.MaxDevices = DefaultDevices
	}
	if cfg.Channels <= 0 {
		cfg.Channels = DefaultChannels
	}
	if cfg.Minutes <= 0 {
		cfg.Minutes = DefaultMinutes
	}
	rng := rand.New(rand.NewSource(cfg.Seed))
	start := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Duration(cfg.Minutes) * time.Minute)
	events := make([]ViewingEvent, 0)
	sequence := 0
	nextID := func() string {
		sequence++
		return fmt.Sprintf("event-%08d", sequence)
	}
	for h := 1; h <= cfg.Households; h++ {
		householdID := fmt.Sprintf("household-%03d", h)
		deviceCount := 1 + rng.Intn(cfg.MaxDevices)
		for d := 1; d <= deviceCount; d++ {
			deviceID := fmt.Sprintf("%s-device-%02d", householdID, d)
			channel := 1 + rng.Intn(cfg.Channels)
			current := start.Add(time.Duration(rng.Intn(30)) * time.Second)
			events = append(events, ViewingEvent{
				EventID: nextID(), Timestamp: current, HouseholdID: householdID,
				DeviceID: deviceID, ChannelID: fmt.Sprintf("channel-%02d", channel),
				EventType: EventViewingStart,
			})
			breakNumber := 0
			for current.Before(end) {
				current = current.Add(time.Duration(10+rng.Intn(6)) * time.Minute)
				if !current.Before(end) {
					break
				}
				breakNumber++
				opportunityID := fmt.Sprintf("%s-opportunity-%03d", deviceID, breakNumber)
				channel = 1 + rng.Intn(cfg.Channels)
				events = append(events, ViewingEvent{
					EventID: nextID(), Timestamp: current, HouseholdID: householdID,
					DeviceID: deviceID, ChannelID: fmt.Sprintf("channel-%02d", channel),
					EventType: EventAdBreak, OpportunityID: opportunityID,
				})
				for slot := 0; slot < SlotsPerPod; slot++ {
					outcomeRoll := rng.Intn(100)
					outcome := OutcomeConfirmed
					eventType := EventPlaybackConfirm
					offset := 90 * time.Second
					switch {
					case outcomeRoll >= 80 && outcomeRoll < 90:
						outcome = OutcomeCanceled
						eventType = EventPlaybackCancel
						offset = 45 * time.Second
					case outcomeRoll >= 90:
						outcome = OutcomeDelayed
						eventType = EventPlaybackDelayed
						offset = 3 * time.Minute
						if slot%2 == 1 {
							// Some delayed callbacks arrive after a later
							// break request and exercise TTL cleanup.
							offset = 13 * time.Minute
						}
					}
					events = append(events, ViewingEvent{
						EventID: nextID(), Timestamp: current.Add(offset),
						HouseholdID: householdID, DeviceID: deviceID,
						ChannelID: fmt.Sprintf("channel-%02d", channel), EventType: eventType,
						OpportunityID: opportunityID, Slot: slot, Outcome: outcome,
					})
				}
				if rng.Intn(4) == 0 {
					switchAt := current.Add(time.Duration(30+rng.Intn(90)) * time.Second)
					if switchAt.Before(end) {
						channel = 1 + rng.Intn(cfg.Channels)
						events = append(events, ViewingEvent{
							EventID: nextID(), Timestamp: switchAt, HouseholdID: householdID,
							DeviceID: deviceID, ChannelID: fmt.Sprintf("channel-%02d", channel),
							EventType: EventChannelSwitch,
						})
					}
				}
			}
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			pi, pj := eventPriority(events[i].EventType), eventPriority(events[j].EventType)
			if pi != pj {
				return pi < pj
			}
			return events[i].EventID < events[j].EventID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events
}

func eventPriority(eventType EventType) int {
	switch eventType {
	case EventChannelSwitch:
		return 10
	case EventViewingStart:
		return 20
	case EventAdBreak:
		return 30
	case EventPlaybackCancel:
		return 40
	case EventPlaybackConfirm:
		return 50
	case EventPlaybackDelayed:
		return 60
	default:
		return 100
	}
}

// Run replays the shared timeline twice with separate stores.
func Run(cfg Config) (Report, error) {
	normalizeConfig(&cfg)
	if len(cfg.Candidates) == 0 {
		_, cfg.Candidates, cfg.Registry, cfg.GroundTruth = fixtures.Generate(cfg.Seed)
	}
	if len(cfg.Candidates) == 0 {
		return Report{}, errors.New("simulator requires candidates")
	}
	predictions := creative.NewResolver(cfg.Registry).ResolveAll(cfg.Candidates)
	events := GenerateEvents(cfg)
	return Report{
		Seed:      cfg.Seed,
		Events:    events,
		Baseline:  replay("baseline", cfg, events, cfg.Candidates, predictions, false),
		Optimized: replay("optimized", cfg, events, cfg.Candidates, predictions, true),
	}, nil
}

func normalizeConfig(cfg *Config) {
	defaults := DefaultConfig()
	if cfg.Seed == 0 {
		cfg.Seed = defaults.Seed
	}
	if cfg.Households <= 0 {
		cfg.Households = defaults.Households
	}
	if cfg.MaxDevices <= 0 {
		cfg.MaxDevices = defaults.MaxDevices
	}
	if cfg.Channels <= 0 {
		cfg.Channels = defaults.Channels
	}
	if cfg.Minutes <= 0 {
		cfg.Minutes = defaults.Minutes
	}
}

type pendingSelection struct {
	reservationID string
	candidate     pod.ResolvedCandidate
}

type confirmedAd struct {
	at        time.Time
	household string
	candidate model.Candidate
}

func replay(name string, cfg Config, events []ViewingEvent, candidates []model.Candidate, predictions []model.ResolvedCreative, optimized bool) StrategyReport {
	simNow := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	store, _ := frequency.NewInMemoryFrequencyStore(FrequencyLimit, FrequencyWindow, func() time.Time { return simNow })
	resolved := make(map[string]pod.ResolvedCandidate, len(predictions))
	for i, prediction := range predictions {
		resolved[prediction.CandidateID] = pod.ResolvedCandidate{Candidate: candidates[i], Creative: prediction}
	}
	confirmed := seedInitialHistory(store, resolved, events, optimized, simNow)
	report := Metrics{}
	for _, prediction := range predictions {
		if prediction.CanonicalCreativeID == "" {
			report.UnresolvedIdentities++
		}
	}
	pending := make(map[string]pendingSelection)
	for _, event := range events {
		simNow = event.Timestamp
		switch event.EventType {
		case EventAdBreak:
			report.ProcessedBreaks++
			report.AdOpportunities += SlotsPerPod
			ordered := make([]pod.ResolvedCandidate, 0, len(candidates))
			for _, candidate := range candidates {
				ordered = append(ordered, resolved[candidate.CandidateID])
			}
			selector := &pod.Selector{
				FrequencyStore: store, FrequencyLimit: FrequencyLimit,
				FrequencyWindow: FrequencyWindow, ReservationTTL: ReservationTTL,
				DedupeCanonical: optimized,
			}
			if !optimized {
				selector.ReservationKey = func(request pod.PodRequest, candidate pod.ResolvedCandidate) string {
					return "source:" + candidate.Candidate.Source + ":" + candidate.Candidate.SourceCreativeID
				}
			}
			householdKey := event.HouseholdID
			if !optimized {
				householdKey = event.DeviceID
			}
			result, err := selector.Select(pod.PodRequest{HouseholdID: householdKey, DurationSeconds: PodDurationSeconds}, ordered)
			if err != nil {
				report.IncompletePods++
				continue
			}
			for slot, selected := range result.Selected {
				report.SelectedAds++
				report.EstimatedRevenue += selected.Candidate.CPM / 1000
				pending[pendingKey(event.OpportunityID, slot)] = pendingSelection{result.ReservationIDs[slot], selected}
			}
			countPodDuplicates(&report, result.Selected, cfg.GroundTruth)
		case EventPlaybackCancel, EventPlaybackConfirm, EventPlaybackDelayed:
			pendingAd, ok := pending[pendingKey(event.OpportunityID, event.Slot)]
			if !ok {
				continue
			}
			switch event.Outcome {
			case OutcomeCanceled:
				if err := store.Cancel(context.Background(), pendingAd.reservationID); err == nil {
					report.CanceledImpressions++
				} else {
					report.ExpiredOrUnknownSettlements++
				}
			case OutcomeConfirmed, OutcomeDelayed:
				if err := store.Confirm(context.Background(), pendingAd.reservationID, event.Timestamp); err == nil {
					report.ConfirmedImpressions++
					if event.Outcome == OutcomeDelayed {
						report.DelayedConfirmations++
					}
					report.ConfirmedRevenue += pendingAd.candidate.Candidate.CPM / 1000
					confirmed = append(confirmed, confirmedAd{event.Timestamp, event.HouseholdID, pendingAd.candidate.Candidate})
				} else {
					report.ExpiredOrUnknownSettlements++
				}
			}
			delete(pending, pendingKey(event.OpportunityID, event.Slot))
		}
	}
	report.FrequencyCapViolations = countCapViolations(confirmed, cfg.GroundTruth)
	if report.AdOpportunities > 0 {
		report.FillRate = float64(report.SelectedAds) / float64(report.AdOpportunities)
	}
	return StrategyReport{Name: name, Metrics: report}
}

func pendingKey(opportunity string, slot int) string {
	return fmt.Sprintf("%s/%d", opportunity, slot)
}

func seedInitialHistory(store *frequency.InMemoryFrequencyStore, resolved map[string]pod.ResolvedCandidate, events []ViewingEvent, optimized bool, at time.Time) []confirmedAd {
	seen := make(map[string]bool)
	confirmed := make([]confirmedAd, 0)
	var historyCandidate pod.ResolvedCandidate
	for _, candidate := range resolved {
		if candidate.Creative.CanonicalCreativeID != "creative-003" {
			continue
		}
		if historyCandidate.Candidate.CandidateID == "" || candidate.Candidate.CandidateID < historyCandidate.Candidate.CandidateID {
			historyCandidate = candidate
		}
	}
	for _, event := range events {
		if event.EventType != EventViewingStart || seen[event.HouseholdID] {
			continue
		}
		seen[event.HouseholdID] = true
		if historyCandidate.Candidate.CandidateID == "" {
			continue
		}
		selected := historyCandidate
		household := event.HouseholdID
		key := "creative-003"
		if !optimized {
			household = event.DeviceID
			key = "source:" + selected.Candidate.Source + ":" + selected.Candidate.SourceCreativeID
		}
		id, ok, _ := store.Reserve(context.Background(), household, key, FrequencyLimit, FrequencyWindow, ReservationTTL)
		if ok {
			_ = store.Confirm(context.Background(), id, at.Add(-30*time.Minute))
			confirmed = append(confirmed, confirmedAd{
				at: at.Add(-30 * time.Minute), household: event.HouseholdID,
				candidate: selected.Candidate,
			})
		}
	}
	return confirmed
}

func countPodDuplicates(report *Metrics, selected []pod.ResolvedCandidate, groundTruth model.ResolutionExpectations) {
	seen := make(map[string]bool)
	for _, candidate := range selected {
		id := groundTruth.Resolved[candidate.Candidate.CandidateID]
		if id == "" {
			continue
		}
		if seen[id] {
			report.IntraPodDuplicates++
			if candidate.Candidate.Replaceable {
				report.AvoidableIntraPodDuplicates++
			} else {
				report.UnavoidableProtectedDuplicates++
			}
		}
		seen[id] = true
	}
}

func countCapViolations(impressions []confirmedAd, groundTruth model.ResolutionExpectations) int {
	type exposure struct{ at time.Time }
	byKey := make(map[string][]exposure)
	sort.SliceStable(impressions, func(i, j int) bool { return impressions[i].at.Before(impressions[j].at) })
	violations := 0
	for _, impression := range impressions {
		canonical := groundTruth.Resolved[impression.candidate.CandidateID]
		if canonical == "" {
			continue
		}
		key := impression.household + "\x00" + canonical
		cutoff := impression.at.Add(-FrequencyWindow)
		exposures := byKey[key][:0]
		for _, prior := range byKey[key] {
			if prior.at.After(cutoff) {
				exposures = append(exposures, prior)
			}
		}
		if len(exposures) >= FrequencyLimit {
			violations++
		}
		exposures = append(exposures, exposure{impression.at})
		byKey[key] = exposures
	}
	return violations
}
