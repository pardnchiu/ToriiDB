package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pardnchiu/toriidb/core/store"
)

const (
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
)

var (
	ErrAlreadyRunning = errors.New("another daemon is serving this socket")
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("bad request")
	ErrNoEmbedder     = store.ErrNoEmbedder
)

type Mode int

const (
	ModeServer Mode = iota
	ModeClient
)

type Daemon struct {
	socket string
	mode   Mode
	torii  *store.Store
	client *client

	mu       sync.Mutex
	wg       sync.WaitGroup
	listener net.Listener
	conns    map[net.Conn]struct{}
	done     chan struct{}
}

func New(dir string, apiKey ...string) (*Daemon, error) {
	if dir == "" {
		return nil, errors.New("dir is required")
	}

	dir, err := resolveDir(dir)
	if err != nil {
		return nil, err
	}

	socket := resolveSocket(dir)

	live, err := probeSocket(socket)
	if err != nil {
		return nil, err
	}

	if live {
		return &Daemon{
			socket: socket,
			mode:   ModeClient,
			client: newClient(socket),
		}, nil
	}

	torii, err := store.New(dir, apiKey...)
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", dir, err)
	}

	return &Daemon{
		socket: socket,
		mode:   ModeServer,
		torii:  torii,
	}, nil
}

func (d *Daemon) Mode() Mode { return d.mode }

func (d *Daemon) Socket() string { return d.socket }

func (d *Daemon) Store() *store.Store { return d.torii }

func (d *Daemon) Start() error {
	if d.mode == ModeClient {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.listener != nil {
		return nil
	}

	listener, err := listenSocket(d.socket)
	if err != nil {
		return err
	}

	d.listener = listener
	d.conns = make(map[net.Conn]struct{})
	d.done = make(chan struct{})

	go d.accept(listener, d.done)

	return nil
}

func (d *Daemon) accept(listener net.Listener, done chan struct{}) {
	defer close(done)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		d.mu.Lock()
		if d.listener != listener {
			d.mu.Unlock()
			conn.Close()
			return
		}
		d.conns[conn] = struct{}{}
		d.wg.Add(1)
		d.mu.Unlock()

		go d.serve(conn)
	}
}

func (d *Daemon) serve(conn net.Conn) {
	defer func() {
		conn.Close()
		d.mu.Lock()
		delete(d.conns, conn)
		d.mu.Unlock()
		d.wg.Done()
	}()

	reader := bufio.NewReader(conn)

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		raw, err := readFrame(reader)
		if err != nil {
			return
		}

		var req request
		res := failure(kindBadRequest, "invalid json payload")
		if json.Unmarshal(raw, &req) == nil {
			res = d.dispatch(context.Background(), &req)
		}

		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := writeFrame(conn, res); err != nil {
			return
		}
	}
}

func (d *Daemon) Stop() error {
	if d.mode == ModeClient {
		return nil
	}

	d.mu.Lock()
	listener, done := d.listener, d.done
	if listener == nil {
		d.mu.Unlock()
		return nil
	}
	d.listener = nil
	d.done = nil
	d.mu.Unlock()

	listener.Close()
	<-done

	d.mu.Lock()
	for conn := range d.conns {
		conn.Close()
	}
	d.mu.Unlock()

	d.wg.Wait()

	return nil
}

func (d *Daemon) Close() error {
	if d.mode == ModeClient {
		d.client.close()
		return nil
	}

	return errors.Join(d.Stop(), d.torii.Close())
}
