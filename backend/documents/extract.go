package documents

import "time"

// Chunk is a slice of a document together with its embedding vector. It is the
// short-term memory cached in the store at upload time and reused at query time.
type Chunk struct {
	Text   string
	Vector []float32
}

type Document struct {
	ID         string    // stable identifier used by the file-manager API
	Name       string    // original filename
	Content    string    // extracted plain text (editable via the API)
	Type       string    // MIME type reported at upload time
	Size       int64     // byte length of Content
	UploadedAt time.Time // when the document was first stored
	Chunks     []Chunk   // text+embedding pairs; regenerated on edit
}

// Function that supports txt file
func Extract(name string, data []byte) Document {
	return Document{Name: name, Content: string(data)}
}
