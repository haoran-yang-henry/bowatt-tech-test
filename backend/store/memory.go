package store

import (
	"bowatt-backend/documents"
	"sync"
)

// document repository, stores the document content when uploading and give back when querying
type Memory struct {
	mu   sync.RWMutex
	docs []documents.Document
}

func NewMemory() *Memory {
	return &Memory{}
}

func (m *Memory) Save(d documents.Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs = append(m.docs, d)
}

func (m *Memory) All() []documents.Document {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]documents.Document, len(m.docs))
	copy(out, m.docs)
	return out
}
