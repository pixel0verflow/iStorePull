package store

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/pixel0verflow/istorepull/pkg/credential"
	"github.com/pixel0verflow/istorepull/pkg/httpx"
)

// downloadPath is the version-selectable product endpoint.
const downloadPath = "/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct"

// Client replays the store endpoints.
type Client interface {
	// DownloadProduct fetches a download ticket. externalVersionID "" = current.
	DownloadProduct(ctx context.Context, adamID int64, externalVersionID string) (DownloadItem, error)
	// Versions lists every downloadable build for a title.
	Versions(ctx context.Context, adamID int64) (VersionList, error)
	// ResolveVersions maps external ids to version strings by probing. only==nil
	// resolves the whole list.
	ResolveVersions(ctx context.Context, vl VersionList, only []string) ([]VersionInfo, error)
	// FindExternalID resolves a version string to its external version id.
	FindExternalID(ctx context.Context, vl VersionList, version string) (string, error)
}

type client struct {
	hx   *httpx.Client
	sess credential.Session
}

// Option configures a store Client.
type Option func(*config)

type config struct {
	httpOpts []httpx.Option
}

// WithHTTPClient injects a custom *http.Client (used by tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) { c.httpOpts = append(c.httpOpts, httpx.WithHTTPClient(hc)) }
}

// WithBaseURL overrides the store base URL (used by tests).
func WithBaseURL(u string) Option {
	return func(c *config) { c.httpOpts = append(c.httpOpts, httpx.WithBaseURL(u)) }
}

// New builds a store Client from a session.
func New(sess credential.Session, opts ...Option) (Client, error) {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	hx, err := httpx.New(sess, cfg.httpOpts...)
	if err != nil {
		return nil, err
	}
	return &client{hx: hx, sess: sess}, nil
}

// guid is bound to the token; query string mirrors Configurator's request.
func (c *client) DownloadProduct(ctx context.Context, adamID int64, externalVersionID string) (DownloadItem, error) {
	body := newDownloadPayload(c.sess.GUID, adamID, externalVersionID)
	var resp productResponse
	path := downloadPath + "?guid=" + c.sess.GUID
	if err := c.hx.PostPlist(ctx, path, body, &resp); err != nil {
		return DownloadItem{}, mapHTTPError(err)
	}
	if resp.FailureType != "" {
		return DownloadItem{}, classify(resp.FailureType, resp.CustomerMessage)
	}
	if len(resp.SongList) == 0 {
		return DownloadItem{}, ErrEmptyResult
	}
	return resp.SongList[0].toDownloadItem(adamID), nil
}

// classify turns a failureType into a typed error, preferring the category
// sentinel so callers can errors.Is against it.
func classify(failureType, msg string) error {
	fe := &FailureError{Type: failureType, Message: msg}
	if cat := classifyFailure(failureType); cat != nil {
		return fmt.Errorf("%w (%s)", cat, fe.Error())
	}
	return fe
}

// mapHTTPError translates transport errors into store sentinels where possible.
func mapHTTPError(err error) error {
	var he *httpx.HTTPError
	if errors.As(err, &he) {
		switch he.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("%w (HTTP %d)", ErrSessionExpired, he.Status)
		case http.StatusNotFound, http.StatusGone:
			return fmt.Errorf("%w (HTTP %d)", ErrNotServed, he.Status)
		}
	}
	return err
}
