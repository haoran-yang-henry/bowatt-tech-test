package agent

import (
	"context"
	"fmt"
)

// For further intergration of agent tools
type Tool struct {
	Name string
	Run  func(ctx context.Context, input string) (string, error)
}

// tool routing function
func route(ctx context.Context, tools []Tool, name, input string) (string, error) {
	for _, t := range tools {
		if t.Name == name {
			return t.Run(ctx, input)
		}
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}
