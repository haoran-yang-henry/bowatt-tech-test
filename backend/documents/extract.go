package documents

// Chunk is a slice of a document together with its embedding vector. It is the
// short-term memory cached in the store at upload time and reused at query time.
type Chunk struct {
	Text   string
	Vector []float32
}

type Document struct {
	Name    string
	Content string
	Chunks  []Chunk
}

// Function that supports txt file
func Extract(name string, data []byte) Document {
	return Document{Name: name, Content: string(data)}
}
