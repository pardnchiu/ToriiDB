package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

const (
	maxIdleConns = 8
	idleWindow   = 5 * time.Second
)

type pooled struct {
	net.Conn
	reader *bufio.Reader
	usedAt time.Time
}

type client struct {
	socket string

	mu   sync.Mutex
	idle []*pooled
}

func newClient(socket string) *client {
	return &client{socket: socket}
}

func (c *client) call(ctx context.Context, req request) (*response, error) {
	if reused := c.take(); reused != nil {
		res, reusable, err := c.exchange(ctx, reused, req)
		if err == nil {
			c.put(reused, reusable)
			return res, nil
		}

		reused.Close()
		if !dropped(err) {
			return nil, err
		}
	}

	fresh, err := c.dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.socket, err)
	}

	res, reusable, err := c.exchange(ctx, fresh, req)
	if err != nil {
		fresh.Close()
		return nil, err
	}

	c.put(fresh, reusable)
	return res, nil
}

func (c *client) exchange(ctx context.Context, p *pooled, req request) (res *response, reusable bool, err error) {
	stop := context.AfterFunc(ctx, func() { p.Close() })
	defer func() { reusable = stop() && err == nil }()

	if deadline, ok := ctx.Deadline(); ok {
		p.SetDeadline(deadline)
	} else {
		p.SetDeadline(time.Time{})
	}

	if err := writeFrame(p, req); err != nil {
		return nil, false, callError(ctx, req.Op, err)
	}

	raw, err := readFrame(p.reader)
	if err != nil {
		return nil, false, callError(ctx, req.Op, err)
	}

	var decoded response
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, false, fmt.Errorf("%s: decode response: %w", req.Op, err)
	}

	if decoded.Error != "" {
		return nil, false, remoteError(decoded.Kind, decoded.Error)
	}

	return &decoded, false, nil
}

func (c *client) dial(ctx context.Context) (*pooled, error) {
	var dialer net.Dialer

	raw, err := dialer.DialContext(ctx, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	return &pooled{Conn: raw, reader: bufio.NewReader(raw)}, nil
}

func (c *client) take() *pooled {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.idle) > 0 {
		last := c.idle[len(c.idle)-1]
		c.idle = c.idle[:len(c.idle)-1]
		if now.Sub(last.usedAt) < idleWindow {
			return last
		}
		last.Close()
	}

	return nil
}

func (c *client) put(p *pooled, reusable bool) {
	if !reusable {
		p.Close()
		return
	}

	p.usedAt = time.Now()

	c.mu.Lock()
	if len(c.idle) >= maxIdleConns {
		c.mu.Unlock()
		p.Close()
		return
	}
	c.idle = append(c.idle, p)
	c.mu.Unlock()
}

func (c *client) close() {
	c.mu.Lock()
	idle := c.idle
	c.idle = nil
	c.mu.Unlock()

	for _, one := range idle {
		one.Close()
	}
}

func dropped(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET)
}

func callError(ctx context.Context, op string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", op, ctxErr)
	}
	return fmt.Errorf("%s: %w", op, err)
}
