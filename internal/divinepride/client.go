// Package divinepride is a polite, cached client for the Divine Pride API.
//
// Usage rules (https://www.divine-pride.net/tools/api-doc): cache responses,
// honor 429 + Retry-After, and never bulk-enumerate IDs or mirror the database.
// This client only fetches single IDs on demand and caches them on disk.
package divinepride

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const baseURL = "https://www.divine-pride.net/api/database"

// Kind is an API entity type.
type Kind string

const (
	Item    Kind = "Item"
	Monster Kind = "Monster"
	Skill   Kind = "Skill"
	Efst    Kind = "Efst"
)

// ErrNotFound is returned for HTTP 404.
var ErrNotFound = errors.New("divinepride: not found")

// Client fetches entities. It is safe for concurrent use.
type Client struct {
	key      string
	server   string // x-server alias, e.g. "LATAM"
	cacheDir string
	http     *http.Client
	minGap   time.Duration

	mu   sync.Mutex
	last time.Time
}

// New returns a client. cacheDir may be empty to disable the disk cache.
func New(apiKey, server, cacheDir string) *Client {
	return &Client{
		key:      apiKey,
		server:   server,
		cacheDir: cacheDir,
		http:     &http.Client{Timeout: 15 * time.Second},
		minGap:   500 * time.Millisecond, // conservative: the real limit is per-key and unpublished
	}
}

// Get returns the raw JSON for one entity in the given language (en, pt, es, ...).
func (c *Client) Get(ctx context.Context, kind Kind, id int, lang string) ([]byte, error) {
	cachePath := ""
	if c.cacheDir != "" {
		cachePath = filepath.Join(c.cacheDir, c.server, lang, string(kind), strconv.Itoa(id)+".json")
		if b, err := os.ReadFile(cachePath); err == nil {
			return b, nil
		}
	}

	u := fmt.Sprintf("%s/%s/%d?apiKey=%s", baseURL, kind, id, url.QueryEscape(c.key))
	for attempt := 0; attempt < 4; attempt++ {
		c.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, errors.New("divinepride: build request")
		}
		req.Header.Set("Accept-Language", lang)
		req.Header.Set("x-server", c.server)

		resp, err := c.http.Do(req)
		if err != nil {
			// Never wrap err: *url.Error contains the URL, which contains the API key.
			return nil, fmt.Errorf("divinepride: %s %d: request failed", kind, id)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			if cachePath != "" {
				_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
				_ = os.WriteFile(cachePath, body, 0o644)
			}
			return body, nil
		case http.StatusNotFound:
			return nil, ErrNotFound
		case http.StatusTooManyRequests:
			delay := 5 * time.Second
			if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
				delay = time.Duration(s) * time.Second
			}
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		case http.StatusUnauthorized:
			return nil, errors.New("divinepride: invalid or missing API key (401)")
		default:
			return nil, fmt.Errorf("divinepride: %s %d: HTTP %d", kind, id, resp.StatusCode)
		}
	}
	return nil, fmt.Errorf("divinepride: %s %d: rate limited, gave up", kind, id)
}

func (c *Client) wait() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.minGap - time.Since(c.last); d > 0 {
		time.Sleep(d)
	}
	c.last = time.Now()
}
