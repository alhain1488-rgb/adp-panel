// Package sshtest provides in-memory SSH mocks for unit tests.
package sshtest

import (
	"context"
	"sync"

	"github.com/adp/panel/internal/ssh"
)

// MockRunner scripts command responses and records what was run.
type MockRunner struct {
	// Handler returns the result for a command. If nil, an empty success is returned.
	Handler func(cmd, input string) (ssh.Result, error)

	mu       sync.Mutex
	Commands []string
	Closed   bool
}

func (m *MockRunner) record(cmd string) {
	m.mu.Lock()
	m.Commands = append(m.Commands, cmd)
	m.mu.Unlock()
}

// Run implements ssh.Runner.
func (m *MockRunner) Run(_ context.Context, cmd string) (ssh.Result, error) {
	m.record(cmd)
	if m.Handler == nil {
		return ssh.Result{}, nil
	}
	return m.Handler(cmd, "")
}

// RunInput implements ssh.Runner.
func (m *MockRunner) RunInput(_ context.Context, cmd, input string) (ssh.Result, error) {
	m.record(cmd)
	if m.Handler == nil {
		return ssh.Result{}, nil
	}
	return m.Handler(cmd, input)
}

// Close implements ssh.Runner.
func (m *MockRunner) Close() error {
	m.Closed = true
	return nil
}

// Ran reports whether any recorded command contains substr.
func (m *MockRunner) Ran(substr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.Commands {
		if contains(c, substr) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// MockDialer returns a preconfigured runner, or an error to simulate an
// unreachable node.
type MockDialer struct {
	Runner  ssh.Runner
	DialErr error

	mu     sync.Mutex
	Dialed []ssh.Target
}

// Dial implements ssh.Dialer.
func (d *MockDialer) Dial(_ context.Context, t ssh.Target) (ssh.Runner, error) {
	d.mu.Lock()
	d.Dialed = append(d.Dialed, t)
	d.mu.Unlock()
	if d.DialErr != nil {
		return nil, d.DialErr
	}
	if d.Runner != nil {
		return d.Runner, nil
	}
	return &MockRunner{}, nil
}
