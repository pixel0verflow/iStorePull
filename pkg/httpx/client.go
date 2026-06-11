package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// maxRedirects bounds the manual pod-redirect loop.
const maxRedirects = 5

// Client is a store-aware HTTP client. It carries the captured cookie jar and
// re-issues POSTs across Apple's pod (pN-buy) redirects instead of letting Go
// downgrade them to GET and drop the body.
type Client struct {
	hc      *http.Client
	sess    credential.Session
	baseURL string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying *http.Client (tests inject httptest).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// WithBaseURL overrides the store base URL (tests point this at httptest).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// DefaultBaseURL is Apple's store host.
const DefaultBaseURL = "https://buy.itunes.apple.com"

// New builds a Client seeded with the session's cookie jar.
func New(sess credential.Session, opts ...Option) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &Client{
		sess:    sess,
		baseURL: DefaultBaseURL,
		hc: &http.Client{
			// Never auto-follow; we handle redirects ourselves to keep the POST.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Jar: jar,
		},
	}
	for _, o := range opts {
		o(c)
	}
	if c.hc.Jar == nil {
		c.hc.Jar = jar
	}
	c.seedCookies()
	return c, nil
}

// seedCookies installs the captured jar onto the cookie jar for apple hosts.
func (c *Client) seedCookies() {
	cookies := c.sess.HTTPCookies()
	if len(cookies) == 0 {
		return
	}
	for _, host := range []string{c.baseURL, "https://apple.com"} {
		if u, err := url.Parse(host); err == nil {
			c.hc.Jar.SetCookies(u, cookies)
		}
	}
}

// PostPlist marshals body to a plist, POSTs to path (relative to the base URL),
// follows pod redirects preserving the body, and decodes the response into out.
func (c *Client) PostPlist(ctx context.Context, path string, body any, out any) error {
	encoded, err := MarshalPlist(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	target := c.baseURL + path
	resp, err := c.postPreservingBody(ctx, target, encoded)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return &HTTPError{Status: resp.StatusCode, Body: data}
	}
	if out == nil {
		return nil
	}
	if err := UnmarshalPlist(data, out); err != nil {
		return fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
	}
	return nil
}

// postPreservingBody POSTs and re-POSTs on 3xx Location, preserving the body.
func (c *Client) postPreservingBody(ctx context.Context, target string, body []byte) (*http.Response, error) {
	for i := 0; i <= maxRedirects; i++ {
		req, err := c.newRequest(ctx, target, body)
		if err != nil {
			return nil, err
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("POST %s: %w", target, err)
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			_ = resp.Body.Close()
			if loc == "" {
				return nil, fmt.Errorf("redirect with no Location from %s", target)
			}
			next, err := resolveLocation(target, loc)
			if err != nil {
				return nil, err
			}
			target = next
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("too many redirects starting at %s", target)
}

func (c *Client) newRequest(ctx context.Context, target string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-apple-plist")
	for k, v := range c.sess.Headers() {
		req.Header.Set(k, v)
	}
	return req, nil
}

// resolveLocation resolves a possibly-relative redirect target against the base.
func resolveLocation(base, loc string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	l, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(l).String(), nil
}

// HTTPError is returned for non-2xx store responses.
type HTTPError struct {
	Status int
	Body   []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("store returned HTTP %d", e.Status)
}
