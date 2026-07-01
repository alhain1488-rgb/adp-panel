// Package ssh runs commands on managed nodes over SSH, behind interfaces so the
// rest of the backend can be unit-tested with a mock (see sshtest).
package ssh

import "context"

// AuthMethod identifies how to authenticate to a node.
type AuthMethod string

const (
	AuthKey      AuthMethod = "key"
	AuthPassword AuthMethod = "password"
)

// Target describes how to reach a node. Secrets are already decrypted here.
type Target struct {
	Host       string
	Port       int
	User       string
	Auth       AuthMethod
	Secret     string // password or PEM private key
	Passphrase string // key passphrase, if any
}

// Result is the outcome of running a command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes commands on a node. Close releases the connection.
type Runner interface {
	// Run executes cmd and returns its result. A non-zero exit code is returned
	// in Result.ExitCode, not as an error; err is for connection/transport failures.
	Run(ctx context.Context, cmd string) (Result, error)
	// RunInput executes cmd with stdin fed from input (used to write files).
	RunInput(ctx context.Context, cmd, input string) (Result, error)
	Close() error
}

// Dialer opens a Runner to a target. The real implementation dials SSH; tests
// inject a mock.
type Dialer interface {
	Dial(ctx context.Context, t Target) (Runner, error)
}
