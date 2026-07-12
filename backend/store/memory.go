package store

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"bowatt-backend/documents"
)

// Memory is an in-process document repository. It stores the extracted content
// and cached embeddings for every uploaded source and serves them back to the
// file-manager API and the research agent. Access is safe for concurrent use.
type Memory struct {
	mu   sync.RWMutex
	docs []documents.Document
}

func NewMemory() *Memory {
	return &Memory{}
}

// Add stores a new document, assigning it an ID if it does not already have one,
// and returns the stored copy (so the caller sees the generated ID).
func (m *Memory) Add(d documents.Document) documents.Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d.ID == "" {
		d.ID = newID()
	}
	m.docs = append(m.docs, d)
	return d
}

// All returns a copy of every stored document.
func (m *Memory) All() []documents.Document {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]documents.Document, len(m.docs))
	copy(out, m.docs)
	return out
}

// Get returns the document with the given ID.
func (m *Memory) Get(id string) (documents.Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.docs {
		if d.ID == id {
			return d, true
		}
	}
	return documents.Document{}, false
}

// ByIDs returns the documents whose IDs are in the given set, preserving store
// order. An empty id set returns nothing (callers treat that as "all").
func (m *Memory) ByIDs(ids []string) []documents.Document {
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []documents.Document
	for _, d := range m.docs {
		if _, ok := want[d.ID]; ok {
			out = append(out, d)
		}
	}
	return out
}

// Update replaces the content and chunks of an existing document, keeping its
// ID, name, type, and upload time. Returns the updated copy.
func (m *Memory) Update(id, content string, chunks []documents.Chunk) (documents.Document, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.docs {
		if m.docs[i].ID == id {
			m.docs[i].Content = content
			m.docs[i].Size = int64(len(content))
			m.docs[i].Chunks = chunks
			return m.docs[i], true
		}
	}
	return documents.Document{}, false
}

// Delete removes the document with the given ID. Returns whether it existed.
func (m *Memory) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.docs {
		if m.docs[i].ID == id {
			m.docs = append(m.docs[:i], m.docs[i+1:]...)
			return true
		}
	}
	return false
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
