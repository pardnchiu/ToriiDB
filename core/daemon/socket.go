package daemon

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

const (
	// * not os.TempDir(): it reads $TMPDIR, which makes the socket identity a function
	// * of (dir, environment) instead of dir alone — a daemon under launchd and a CLI in
	// * a terminal would derive different sockets for the same data directory
	socketRoot   = "/tmp"
	probeTimeout = 500 * time.Millisecond
)

func resolveDir(dir string) (string, error) {
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return "", err
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", dir, err)
	}

	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", abs, err)
	}

	return real, nil
}

func resolveSocket(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(socketRoot, fmt.Sprintf("toriidb-%d-%x.sock", os.Getuid(), sum[:8]))
}

func probeSocket(path string) (bool, error) {
	conn, err := net.DialTimeout("unix", path, probeTimeout)
	if err == nil {
		conn.Close()
		return true, nil
	}

	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		return false, nil
	}

	return false, fmt.Errorf("probe socket %s: %w", path, err)
}

func listenSocket(path string) (net.Listener, error) {
	live, err := probeSocket(path)
	if err != nil {
		return nil, err
	}
	if live {
		return nil, ErrAlreadyRunning
	}

	if err := go_pkg_filesystem.Remove(path); err != nil {
		return nil, fmt.Errorf("remove stale socket %s: %w", path, err)
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}

	return listener, nil
}
