package manticore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	_ "github.com/go-sql-driver/mysql"
)

type peerNode struct {
	addr string
	db   *sql.DB
}

// Client manages multi-peer connections to Manticore Search nodes,
// supporting broadcast writes and balanced round-robin reads.
type Client struct {
	cfg     Config
	peers   []*peerNode
	readIdx uint64
	mu      sync.RWMutex
}

// NewClient initializes a Manticore client connected to the configured peers.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	nodes := make([]*peerNode, 0, len(cfg.Peers))
	for _, peerAddr := range cfg.Peers {
		// DSN format for Manticore Search MySQL protocol
		dsn := fmt.Sprintf("@tcp(%s)/?charset=utf8&interpolateParams=true&timeout=%s&readTimeout=%s&writeTimeout=%s",
			peerAddr, cfg.Timeout, cfg.Timeout, cfg.Timeout)

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			// Close previously opened connections on failure
			for _, n := range nodes {
				_ = n.db.Close()
			}
			return nil, fmt.Errorf("manticore: failed to open connection to peer %s: %w", peerAddr, err)
		}

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}

		nodes = append(nodes, &peerNode{
			addr: peerAddr,
			db:   db,
		})
	}

	client := &Client{
		cfg:   cfg,
		peers: nodes,
	}

	return client, nil
}

// Peers returns the configured peer addresses.
func (c *Client) Peers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]string, len(c.peers))
	for i, p := range c.peers {
		res[i] = p.addr
	}
	return res
}

// Ping checks the connectivity to all configured peers.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var firstErr error
	for _, peer := range c.peers {
		if err := peer.db.PingContext(ctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("peer %s ping failed: %w", peer.addr, err)
			}
		}
	}
	return firstErr
}

// Broadcast executes a mutating SQL statement (INSERT, REPLACE, UPDATE, DELETE)
// concurrently across all configured peer nodes.
func (c *Client) Broadcast(ctx context.Context, query string, args ...any) error {
	c.mu.RLock()
	peers := c.peers
	c.mu.RUnlock()

	if len(peers) == 0 {
		return errors.New("manticore: no active peers available")
	}

	errChan := make(chan error, len(peers))
	var wg sync.WaitGroup

	for _, p := range peers {
		wg.Add(1)
		go func(node *peerNode) {
			defer wg.Done()
			_, err := node.db.ExecContext(ctx, query, args...)
			if err != nil {
				errChan <- fmt.Errorf("peer %s exec error: %w", node.addr, err)
				return
			}
			errChan <- nil
		}(p)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// If all peers failed, fail-fast
		if len(errs) == len(peers) {
			return fmt.Errorf("manticore: broadcast failed on all %d peers: %w", len(peers), errs[0])
		}
		// Partial failure is also reported (Zero Fallback)
		return fmt.Errorf("manticore: broadcast failed on %d of %d peers: %w", len(errs), len(peers), errs[0])
	}

	return nil
}

// getReadPeer selects a healthy peer node using round-robin balancing.
func (c *Client) getReadPeer() *peerNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.peers) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&c.readIdx, 1) % uint64(len(c.peers))
	return c.peers[idx]
}

// QueryRow executes a query on a balanced peer, returning a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) (*sql.Row, error) {
	peer := c.getReadPeer()
	if peer == nil {
		return nil, errors.New("manticore: no active peers for reading")
	}
	return peer.db.QueryRowContext(ctx, query, args...), nil
}

// Query executes a query on a balanced peer, returning rows.
func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	peer := c.getReadPeer()
	if peer == nil {
		return nil, errors.New("manticore: no active peers for reading")
	}
	return peer.db.QueryContext(ctx, query, args...)
}

// Close closes all database connection pools across all peers.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for _, p := range c.peers {
		if err := p.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.peers = nil
	return firstErr
}
