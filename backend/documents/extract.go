package documents

type Document struct {
	Name    string
	Content string
}

// Function that supports txt file
func Extract(name string, data []byte) Document {
	return Document{Name: name, Content: string(data)}
}
