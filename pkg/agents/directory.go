package agents

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

var ErrAgentNotFound = errors.New("agent not found")

type Profile struct {
	ID           string
	Role         string
	Capabilities []string
	Status       string
}

type CapabilityMatch struct {
	Profile             Profile
	Score               float64
	MatchedCapabilities []string
	MissingCapabilities []string
}

type Directory interface {
	Register(profile Profile) error
	FindByCapabilities(role string, capabilities []string, limit int) ([]CapabilityMatch, error)
	ClaimByCapabilities(role string, capabilities []string) (CapabilityMatch, error)
	GetProfile(agentID string) (Profile, error)
	UpdateStatus(agentID string, status string) error
}

type MemoryDirectory struct {
	mu       sync.RWMutex
	profiles map[string]Profile
}

func NewMemoryDirectory() *MemoryDirectory {
	return &MemoryDirectory{profiles: map[string]Profile{}}
}

func (d *MemoryDirectory) Register(profile Profile) error {
	if strings.TrimSpace(profile.ID) == "" {
		return errors.New("agent id is required")
	}
	if strings.TrimSpace(profile.Role) == "" {
		return errors.New("agent role is required")
	}
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Role = strings.TrimSpace(strings.ToLower(profile.Role))
	profile.Status = normalizeStatus(profile.Status)
	profile.Capabilities = normalizeCapabilities(profile.Capabilities)

	d.mu.Lock()
	defer d.mu.Unlock()
	d.profiles[profile.ID] = profile
	return nil
}

func (d *MemoryDirectory) FindByCapabilities(role string, capabilities []string, limit int) ([]CapabilityMatch, error) {
	return d.findByCapabilities(role, capabilities, limit, true)
}

func (d *MemoryDirectory) ClaimByCapabilities(role string, capabilities []string) (CapabilityMatch, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	matches := d.findMatchesLocked(role, capabilities, 1, false)
	if len(matches) == 0 {
		return CapabilityMatch{}, ErrAgentNotFound
	}
	selected := matches[0]
	profile := selected.Profile
	profile.Status = "busy"
	d.profiles[profile.ID] = profile
	selected.Profile = profile
	return selected, nil
}

func (d *MemoryDirectory) GetProfile(agentID string) (Profile, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	profile, exists := d.profiles[strings.TrimSpace(agentID)]
	if !exists {
		return Profile{}, ErrAgentNotFound
	}
	return profile, nil
}

func (d *MemoryDirectory) UpdateStatus(agentID string, status string) error {
	trimmedID := strings.TrimSpace(agentID)
	if trimmedID == "" {
		return errors.New("agent id is required")
	}
	trimmedStatus := normalizeStatus(status)

	d.mu.Lock()
	defer d.mu.Unlock()
	profile, exists := d.profiles[trimmedID]
	if !exists {
		return ErrAgentNotFound
	}
	profile.Status = trimmedStatus
	d.profiles[trimmedID] = profile
	return nil
}

func (d *MemoryDirectory) findByCapabilities(role string, capabilities []string, limit int, includeBusy bool) ([]CapabilityMatch, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	required := normalizeCapabilities(capabilities)

	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.findMatchesLocked(role, required, limit, includeBusy), nil
}

func (d *MemoryDirectory) findMatchesLocked(role string, capabilities []string, limit int, includeBusy bool) []CapabilityMatch {
	matches := make([]CapabilityMatch, 0, len(d.profiles))
	for _, profile := range d.profiles {
		if role != "" && profile.Role != role {
			continue
		}
		if !isSelectableStatus(profile.Status, includeBusy) {
			continue
		}
		matched, missing := splitCapabilities(capabilities, profile.Capabilities)
		if len(matched) == 0 {
			continue
		}
		score := capabilityScore(capabilities, matched)
		matches = append(matches, CapabilityMatch{
			Profile:             profile,
			Score:               score,
			MatchedCapabilities: matched,
			MissingCapabilities: missing,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Profile.ID < matches[j].Profile.ID
		}
		return matches[i].Score > matches[j].Score
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func normalizeCapabilities(capabilities []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		trimmed := strings.TrimSpace(strings.ToLower(capability))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func splitCapabilities(required []string, offered []string) ([]string, []string) {
	offeredSet := map[string]struct{}{}
	for _, capability := range normalizeCapabilities(offered) {
		offeredSet[capability] = struct{}{}
	}
	matched := make([]string, 0, len(required))
	missing := make([]string, 0, len(required))
	for _, capability := range required {
		if _, exists := offeredSet[capability]; exists {
			matched = append(matched, capability)
			continue
		}
		missing = append(missing, capability)
	}
	return matched, missing
}

func capabilityScore(required []string, matched []string) float64 {
	if len(required) == 0 {
		return 0
	}
	return float64(len(matched)) / float64(len(required))
}

func normalizeStatus(status string) string {
	trimmed := strings.TrimSpace(strings.ToLower(status))
	if trimmed == "" {
		return "active"
	}
	return trimmed
}

func isSelectableStatus(status string, includeBusy bool) bool {
	switch normalizeStatus(status) {
	case "active", "idle":
		return true
	case "busy":
		return includeBusy
	default:
		return false
	}
}
