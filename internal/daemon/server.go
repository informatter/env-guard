package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Daemon struct {
	vault      SecretProvider
	config     ConfigProvider
	server     *http.Server
	listener   net.Listener
	socketPath string
	mux        *http.ServeMux
}

// New creates a new Daemon instance with the given secret provider, config provider, and socket path.
func New(vault SecretProvider, config ConfigProvider, socketPath string) *Daemon {
	d := &Daemon{
		vault:      vault,
		config:     config,
		socketPath: socketPath,
		mux:        http.NewServeMux(),
	}
	d.mux.HandleFunc("/health", d.healthHandler)
	d.mux.HandleFunc("/secrets", d.secretsHandler)
	return d
}

// Start starts the daemon by listening on the Unix domain socket and serving HTTP requests.
// It creates the socket file with permissions 0700. If the socket file already exists, it is removed first.
func (d *Daemon) Start() error {
	dir := filepath.Dir(d.socketPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	if err := os.Remove(d.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing old socket: %w", err)
	}

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listening on unix socket: %w", err)
	}
	d.listener = listener

	if err := os.Chmod(d.socketPath, 0o700); err != nil {
		listener.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	d.server = &http.Server{
		Handler: d.mux,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			cred := extractCred(c)
			if cred != nil {
				ctx = context.WithValue(ctx, peerCredKey, cred)
			}
			return ctx
		},
	}

	go func() {
		d.server.Serve(listener)
	}()

	return nil
}

func (d *Daemon) Stop() error {
	if d.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.server.Shutdown(ctx)
	}
	os.Remove(d.socketPath)
	return nil
}

func (d *Daemon) SocketPath() string {
	return d.socketPath
}
