package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	xssh "golang.org/x/crypto/ssh"
)

// dialTimeout bounds the TCP + handshake time.
const dialTimeout = 15 * time.Second

// NewDialer returns the production SSH dialer.
func NewDialer() Dialer { return &sshDialer{} }

type sshDialer struct{}

func (d *sshDialer) Dial(ctx context.Context, t Target) (Runner, error) {
	auth, err := authMethods(t)
	if err != nil {
		return nil, err
	}
	cfg := &xssh.ClientConfig{
		User: t.User,
		Auth: auth,
		// Personal panel connecting to the operator's own nodes; host keys are
		// not pinned in MVP.
		HostKeyCallback: xssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         dialTimeout,
	}

	port := t.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(t.Host, strconv.Itoa(port))

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := xssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh: handshake: %w", err)
	}
	return &sshRunner{client: xssh.NewClient(sshConn, chans, reqs)}, nil
}

func authMethods(t Target) ([]xssh.AuthMethod, error) {
	switch t.Auth {
	case AuthPassword:
		return []xssh.AuthMethod{xssh.Password(t.Secret)}, nil
	case AuthKey:
		var signer xssh.Signer
		var err error
		if t.Passphrase != "" {
			signer, err = xssh.ParsePrivateKeyWithPassphrase([]byte(t.Secret), []byte(t.Passphrase))
		} else {
			signer, err = xssh.ParsePrivateKey([]byte(t.Secret))
		}
		if err != nil {
			return nil, fmt.Errorf("ssh: parse private key: %w", err)
		}
		return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("ssh: unknown auth method %q", t.Auth)
	}
}

type sshRunner struct {
	client *xssh.Client
}

func (r *sshRunner) Run(ctx context.Context, cmd string) (Result, error) {
	return r.run(ctx, cmd, "")
}

func (r *sshRunner) RunInput(ctx context.Context, cmd, input string) (Result, error) {
	return r.run(ctx, cmd, input)
}

func (r *sshRunner) run(ctx context.Context, cmd, input string) (Result, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("ssh: new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if input != "" {
		session.Stdin = bytes.NewBufferString(input)
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(xssh.SIGKILL)
		_ = session.Close()
		return Result{}, ctx.Err()
	case err := <-done:
		res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if err == nil {
			return res, nil
		}
		var exitErr *xssh.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitStatus()
			return res, nil
		}
		return res, fmt.Errorf("ssh: run: %w", err)
	}
}

func (r *sshRunner) Close() error {
	return r.client.Close()
}
