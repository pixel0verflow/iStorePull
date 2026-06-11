// Package itunes queries Apple's public iTunes lookup/search API. No credentials
// are needed; it resolves bundle ids, adam ids and basic app metadata.
package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// baseURL is Apple's public iTunes API host.
const baseURL = "https://itunes.apple.com"

// App is a public app record.
type App struct {
	ID       int64   `json:"trackId"`
	BundleID string  `json:"bundleId"`
	Name     string  `json:"trackName"`
	Version  string  `json:"version"`
	Price    float64 `json:"price"`
	Seller   string  `json:"sellerName"`
}

type response struct {
	ResultCount int   `json:"resultCount"`
	Results     []App `json:"results"`
}

// Client queries the public API.
type Client struct {
	hc      *http.Client
	baseURL string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying client (tests inject httptest).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// WithBaseURL overrides the API base (tests point this at httptest).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// New builds a public API client.
func New(opts ...Option) *Client {
	c := &Client{hc: http.DefaultClient, baseURL: baseURL}
	for _, o := range opts {
		o(c)
	}
	return c
}

// LookupBundle resolves a bundle id to its app record.
func (c *Client) LookupBundle(ctx context.Context, bundleID, country string) (App, error) {
	q := url.Values{"bundleId": {bundleID}, "entity": {"software"}}
	if country != "" {
		q.Set("country", country)
	}
	return c.one(ctx, "/lookup?"+q.Encode())
}

// LookupID resolves an adam id to its app record.
func (c *Client) LookupID(ctx context.Context, adamID int64, country string) (App, error) {
	q := url.Values{"id": {strconv.FormatInt(adamID, 10)}, "entity": {"software"}}
	if country != "" {
		q.Set("country", country)
	}
	return c.one(ctx, "/lookup?"+q.Encode())
}

// Search returns up to limit software results for term.
func (c *Client) Search(ctx context.Context, term, country string, limit int) ([]App, error) {
	q := url.Values{"term": {term}, "entity": {"software"}, "media": {"software"}}
	if country != "" {
		q.Set("country", country)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	r, err := c.get(ctx, "/search?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return r.Results, nil
}

func (c *Client) one(ctx context.Context, path string) (App, error) {
	r, err := c.get(ctx, path)
	if err != nil {
		return App{}, err
	}
	if len(r.Results) == 0 {
		return App{}, fmt.Errorf("no result")
	}
	return r.Results[0], nil
}

func (c *Client) get(ctx context.Context, path string) (response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return response{}, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return response{}, fmt.Errorf("itunes API HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, err
	}
	var r response
	if err := json.Unmarshal(data, &r); err != nil {
		return response{}, fmt.Errorf("parse itunes response: %w", err)
	}
	return r, nil
}
