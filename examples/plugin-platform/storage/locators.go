package storage

import (
	"sync"
	"time"
)

type ElementLocator struct {
	Selector   string            `json:"selector"`
	XPath      string            `json:"xpath,omitempty"`
	Type       string            `json:"type,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
}

// ElementLocators stores the selectors needed by the plugin.
type ElementLocators struct {
	Input       ElementLocator  `json:"input"`
	SendButton  ElementLocator  `json:"sendButton"`
	ReplyArea   ElementLocator  `json:"replyArea"`
	CreateChat  *ElementLocator `json:"createChat,omitempty"`
	PlatformURL string          `json:"platformUrl,omitempty"`
	CreatedAt   int64           `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt,omitempty"`
}

// LocatorStore keeps locators in memory by config ID.
type LocatorStore struct {
	mu    sync.RWMutex
	items map[string]ElementLocators
}

func NewLocatorStore() *LocatorStore {
	return &LocatorStore{
		items: make(map[string]ElementLocators),
	}
}

func (s *LocatorStore) Set(configID string, locators ElementLocators) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if locators.UpdatedAt.IsZero() {
		locators.UpdatedAt = time.Now()
	}
	s.items[configID] = locators
}

func (s *LocatorStore) Get(configID string) (ElementLocators, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	locators, ok := s.items[configID]
	return locators, ok
}

func (s *LocatorStore) All() map[string]ElementLocators {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]ElementLocators, len(s.items))
	for id, locators := range s.items {
		copyMap[id] = locators
	}
	return copyMap
}
