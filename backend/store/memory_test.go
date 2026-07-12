package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"bowatt-backend/documents"
)

func doc(name, content string) documents.Document {
	return documents.Document{
		Name:       name,
		Content:    content,
		Type:       "text/plain",
		Size:       int64(len(content)),
		UploadedAt: time.Now().UTC(),
		Chunks:     []documents.Chunk{{Text: content, Vector: []float32{1, 2, 3}}},
	}
}

func TestAddAssignsUniqueIDs(t *testing.T) {
	m := NewMemory()
	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		stored := m.Add(doc(fmt.Sprintf("f%d.txt", i), "content"))
		if stored.ID == "" {
			t.Fatal("Add returned a document without an ID")
		}
		if _, dup := seen[stored.ID]; dup {
			t.Fatalf("Add produced duplicate ID %q", stored.ID)
		}
		seen[stored.ID] = struct{}{}
	}
}

func TestAddKeepsExistingID(t *testing.T) {
	m := NewMemory()
	d := doc("a.txt", "content")
	d.ID = "fixed-id"
	if stored := m.Add(d); stored.ID != "fixed-id" {
		t.Fatalf("Add replaced a preset ID: got %q", stored.ID)
	}
}

func TestGet(t *testing.T) {
	m := NewMemory()
	stored := m.Add(doc("a.txt", "hello"))

	got, ok := m.Get(stored.ID)
	if !ok {
		t.Fatal("Get did not find a stored document")
	}
	if got.Name != "a.txt" || got.Content != "hello" {
		t.Fatalf("Get returned wrong document: %+v", got)
	}

	if _, ok := m.Get("missing"); ok {
		t.Fatal("Get reported a document for an unknown ID")
	}
}

func TestByIDs(t *testing.T) {
	m := NewMemory()
	a := m.Add(doc("a.txt", "alpha"))
	b := m.Add(doc("b.txt", "beta"))
	c := m.Add(doc("c.txt", "gamma"))

	// Selection order must not matter: results come back in store order.
	got := m.ByIDs([]string{c.ID, a.ID})
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != c.ID {
		t.Fatalf("ByIDs returned wrong documents/order: %+v", got)
	}

	if got := m.ByIDs([]string{"missing", b.ID}); len(got) != 1 || got[0].ID != b.ID {
		t.Fatalf("ByIDs did not ignore the unknown ID: %+v", got)
	}

	// Empty selection means "nothing" at this layer; the REST handler is the
	// one that treats an empty selection as "all sources".
	if got := m.ByIDs(nil); len(got) != 0 {
		t.Fatalf("ByIDs(nil) should be empty, got %+v", got)
	}
}

func TestUpdateRewritesContentAndPreservesIdentity(t *testing.T) {
	m := NewMemory()
	stored := m.Add(doc("a.txt", "old content"))

	newChunks := []documents.Chunk{{Text: "new content", Vector: []float32{9}}}
	updated, ok := m.Update(stored.ID, "new content", newChunks)
	if !ok {
		t.Fatal("Update did not find the stored document")
	}

	if updated.Content != "new content" {
		t.Fatalf("content not updated: %q", updated.Content)
	}
	if updated.Size != int64(len("new content")) {
		t.Fatalf("size not recomputed: %d", updated.Size)
	}
	if len(updated.Chunks) != 1 || updated.Chunks[0].Text != "new content" {
		t.Fatalf("chunks not replaced: %+v", updated.Chunks)
	}

	// Identity fields must survive an edit.
	if updated.ID != stored.ID || updated.Name != stored.Name ||
		updated.Type != stored.Type || !updated.UploadedAt.Equal(stored.UploadedAt) {
		t.Fatalf("identity fields changed: before %+v after %+v", stored, updated)
	}

	if _, ok := m.Update("missing", "x", nil); ok {
		t.Fatal("Update reported success for an unknown ID")
	}
}

func TestDelete(t *testing.T) {
	m := NewMemory()
	a := m.Add(doc("a.txt", "alpha"))
	b := m.Add(doc("b.txt", "beta"))

	if !m.Delete(a.ID) {
		t.Fatal("Delete did not find the stored document")
	}
	if _, ok := m.Get(a.ID); ok {
		t.Fatal("document still retrievable after Delete")
	}
	if all := m.All(); len(all) != 1 || all[0].ID != b.ID {
		t.Fatalf("Delete removed the wrong document: %+v", all)
	}

	if m.Delete(a.ID) {
		t.Fatal("Delete reported success for an already-deleted ID")
	}
}

func TestAllReturnsACopy(t *testing.T) {
	m := NewMemory()
	stored := m.Add(doc("a.txt", "alpha"))

	out := m.All()
	out[0].Name = "mutated.txt"

	got, _ := m.Get(stored.ID)
	if got.Name != "a.txt" {
		t.Fatalf("mutating All()'s result leaked into the store: %q", got.Name)
	}
}

// TestConcurrentAccess exercises every method from many goroutines at once.
// It has a deterministic outcome (odd-indexed documents survive) and is mainly
// meaningful under `go test -race`, which CI runs.
func TestConcurrentAccess(t *testing.T) {
	m := NewMemory()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d := m.Add(doc(fmt.Sprintf("f%d.txt", i), "content"))
			m.Get(d.ID)
			m.All()
			m.ByIDs([]string{d.ID})
			if i%2 == 0 {
				m.Delete(d.ID)
			}
		}(i)
	}
	wg.Wait()

	if got := len(m.All()); got != 25 {
		t.Fatalf("expected 25 surviving documents, got %d", got)
	}
}
