package auth

import (
	"errors"
	"sort"
	"sync"
)

// Selector picks a credential for a provider.
type Selector interface {
	Pick(provider string, creds []*Credential) (*Credential, error)
}

// RoundRobinSelector cycles through credentials per provider.
type RoundRobinSelector struct {
	mu      sync.Mutex
	cursors map[string]int
}

func (s *RoundRobinSelector) Pick(provider string, creds []*Credential) (*Credential, error) {
	if len(creds) == 0 {
		return nil, errors.New("selector: no credentials")
	}
	available := filterAvailable(creds)
	if len(available) == 0 {
		return nil, errors.New("selector: no available credentials")
	}
	sort.Slice(available, func(i, j int) bool { return available[i].ID < available[j].ID })
	key := provider
	s.mu.Lock()
	if s.cursors == nil {
		s.cursors = make(map[string]int)
	}
	idx := s.cursors[key]
	s.cursors[key] = idx + 1
	s.mu.Unlock()
	return available[idx%len(available)], nil
}

// PrioritySelector picks the highest priority credential.
type PrioritySelector struct{}

func (s PrioritySelector) Pick(provider string, creds []*Credential) (*Credential, error) {
	if len(creds) == 0 {
		return nil, errors.New("selector: no credentials")
	}
	available := filterAvailable(creds)
	if len(available) == 0 {
		return nil, errors.New("selector: no available credentials")
	}
	sort.Slice(available, func(i, j int) bool {
		if available[i].Priority == available[j].Priority {
			return available[i].ID < available[j].ID
		}
		return available[i].Priority > available[j].Priority
	})
	return available[0], nil
}

func filterAvailable(creds []*Credential) []*Credential {
	available := make([]*Credential, 0, len(creds))
	for _, c := range creds {
		if c == nil || c.Disabled {
			continue
		}
		available = append(available, c)
	}
	return available
}
