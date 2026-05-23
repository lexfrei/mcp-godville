package godville

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
)

const (
	httpClientTimeout    = 30 * time.Second
	errBodyTruncateLimit = 200
)

// userAgent is set by main.go at startup via SetUserAgent so the value
// includes the build version + revision (helpful to Godville ops for
// per-version traffic correlation). Defaults to the bare name for tests
// that construct the client directly.
var userAgent = "mcp-godville" //nolint:gochecknoglobals // build-time-set User-Agent string.

// SetUserAgent overrides the global User-Agent used by every Client.
// Call from main.go after parsing ldflags-injected version/revision.
func SetUserAgent(ua string) {
	if ua != "" {
		userAgent = ua
	}
}

// ErrEmptyGodname is returned when GetHero is called without a godname.
var ErrEmptyGodname = errors.New("godname is required")

// ErrAPI represents an error returned by the Godville API.
var ErrAPI = errors.New("godville API error")

// Client is an HTTP client for the Godville public/private API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new Godville API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: httpClientTimeout},
	}
}

// GetHero fetches hero information. If userkey is empty, the public endpoint
// is used and only public fields are returned. With userkey, private fields
// such as diary, health, and quest progress are also returned.
func (c *Client) GetHero(ctx context.Context, godname, userkey string) (*Hero, error) {
	if godname == "" {
		return nil, ErrEmptyGodname
	}

	reqURL := c.heroURL(godname, userkey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, scrubURLError(err, "request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read body")
	}

	hero, parseErr := parseHero(resp.StatusCode, body)
	if parseErr != nil {
		return nil, parseErr
	}

	return hero, nil
}

func (c *Client) heroURL(godname, userkey string) string {
	path := "/gods/api/" + url.PathEscape(godname)
	if userkey != "" {
		path += "/" + url.PathEscape(userkey)
	}

	return c.baseURL + path + ".json"
}

func parseHero(status int, body []byte) (*Hero, error) {
	// Only probe for the in-body "error" key if the body looks like a JSON
	// object. A non-object body (string, array, number) would fail the
	// probe silently and then surface as the less-useful "decode hero"
	// error. This keeps the probe assumption ("error fields only appear
	// in object responses") explicit instead of relying on probeErr
	// happening to be nil.
	if isJSONObject(body) {
		var probe ErrorPayload

		probeErr := json.Unmarshal(body, &probe)
		if probeErr == nil && probe.Error != "" {
			return nil, errors.Wrapf(ErrAPI, "%s", probe.Error)
		}
	}

	if status >= http.StatusBadRequest {
		// Body might be HTML or text — surface as truncated message.
		return nil, errors.Wrapf(ErrAPI, "status %d: %s", status, truncate(string(body), errBodyTruncateLimit))
	}

	var hero Hero

	err := json.Unmarshal(body, &hero)
	if err != nil {
		return nil, errors.Wrap(err, "decode hero")
	}

	// Preserve raw payload so the raw tool and any field we have not modelled
	// remain accessible. Two forms: a generic map for typed access (lossy on
	// large integers, since encoding/json decodes numbers as float64) and the
	// original bytes for byte-exact inspection.
	rawErr := json.Unmarshal(body, &hero.Raw)
	if rawErr != nil {
		return nil, errors.Wrap(rawErr, "decode raw payload")
	}

	hero.RawBytes = append([]byte(nil), body...)

	return &hero, nil
}

// isJSONObject reports whether body starts with '{' after any leading
// whitespace. Used to gate the in-body error probe so it only runs on
// object-shaped payloads.
func isJSONObject(body []byte) bool {
	trimmed := bytes.TrimLeft(body, " \t\n\r")

	return len(trimmed) > 0 && trimmed[0] == '{'
}

// scrubURLError wraps an http.Client error in a form that does NOT leak the
// request URL. The Godville API requires the userkey to live inside the URL
// path, so any *url.Error from http.Client.Do contains the userkey verbatim
// in its message — which would then propagate through logs and MCP tool
// errors to the caller's stderr. Strip the URL, keep the inner cause.
func scrubURLError(err error, prefix string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return errors.Wrapf(urlErr.Err, "%s (%s)", prefix, urlErr.Op)
	}

	return errors.Wrap(err, prefix)
}

// truncate returns the first n bytes of s with an ellipsis appended. Slices
// at a rune boundary so Russian (or any other multi-byte) error bodies
// produced by the Godville API are not corrupted into invalid UTF-8 when
// they flow through error.Error() into logs and MCP payloads.
func truncate(body string, limit int) string {
	if len(body) <= limit {
		return body
	}

	cut := limit
	for cut > 0 && !utf8.RuneStart(body[cut]) {
		cut--
	}

	return body[:cut] + "…"
}
