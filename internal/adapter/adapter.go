package adapter

import "context"

// Handler processes a user message and returns the agent's response.
type Handler func(ctx context.Context, userMessage string) (string, error)

// Adapter is an external interface that connects users to GoEmon.
// Each adapter runs as a long-lived goroutine started by goemon serve.
type Adapter interface {
	// Name returns the adapter's identifier (e.g., "telegram", "web").
	Name() string

	// Start begins listening for messages. It blocks until ctx is cancelled.
	// The handler function should be called for each incoming user message.
	Start(ctx context.Context, handler Handler) error

	// Send pushes a message to the adapter's default destination (e.g. allowed users).
	Send(ctx context.Context, message string) error

	// Stop gracefully shuts down the adapter.
	Stop() error
}
