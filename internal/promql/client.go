package promql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// service is the SigV4 service name for the CloudWatch monitoring endpoint.
const service = "monitoring"

// Client talks to the CloudWatch PromQL Prometheus-compatible HTTP API,
// signing every request with SigV4 (service "monitoring").
type Client struct {
	Region   string
	baseURL  string
	http     *http.Client
	signer   *v4.Signer
	credsPvd aws.CredentialsProvider
}

// NewClient builds a client for the given region using the supplied credential
// provider (typically from awscfg.Load).
func NewClient(region string, creds aws.CredentialsProvider) *Client {
	return &Client{
		Region:   region,
		baseURL:  fmt.Sprintf("https://monitoring.%s.amazonaws.com", region),
		http:     &http.Client{Timeout: 30 * time.Second},
		signer:   v4.NewSigner(),
		credsPvd: creds,
	}
}

// do signs and executes a request, returning the decoded envelope. Form params
// are sent as an x-www-form-urlencoded POST body (the API's preferred form for
// selectors with special characters).
func (c *Client) do(ctx context.Context, method, path string, form url.Values) (*Envelope, error) {
	var body io.Reader
	var payload string
	if method == http.MethodPost && form != nil {
		payload = form.Encode()
		body = strings.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	sum := sha256.Sum256([]byte(payload))
	payloadHash := hex.EncodeToString(sum[:])

	creds, err := c.credsPvd.Retrieve(ctx)
	if err != nil {
		return nil, &CredentialsError{err: err}
	}
	if err := c.signer.SignHTTP(ctx, creds, req, payloadHash, service, c.Region, time.Now()); err != nil {
		return nil, fmt.Errorf("sigv4 signing: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// Non-JSON error body (e.g. auth failure at the edge).
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return nil, &CredentialsError{err: fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))}
		}
		return nil, fmt.Errorf("decode response (http %d): %w: %s", resp.StatusCode, err, truncate(string(raw), 200))
	}

	if env.Status != "success" {
		return &env, &APIError{HTTPStatus: resp.StatusCode, ErrorType: env.ErrorType, Message: env.Error}
	}
	return &env, nil
}

// Query runs an instant query at an optional evaluation time (zero = now).
func (c *Client) Query(ctx context.Context, expr string, at time.Time, limit int) (*QueryData, []string, error) {
	form := url.Values{"query": {expr}}
	if !at.IsZero() {
		form.Set("time", fmt.Sprintf("%d", at.Unix()))
	}
	if limit > 0 {
		form.Set("limit", fmt.Sprintf("%d", limit))
	}
	env, err := c.do(ctx, http.MethodPost, "/api/v1/query", form)
	if err != nil {
		return nil, warnings(env), err
	}
	var data QueryData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, env.Warnings, err
	}
	return &data, env.Warnings, nil
}

// QueryRange runs a range query, returning a matrix suitable for charting.
func (c *Client) QueryRange(ctx context.Context, expr string, start, end time.Time, step time.Duration, limit int) (*QueryData, []string, error) {
	form := url.Values{
		"query": {expr},
		"start": {fmt.Sprintf("%d", start.Unix())},
		"end":   {fmt.Sprintf("%d", end.Unix())},
		"step":  {fmt.Sprintf("%ds", int(step.Seconds()))},
	}
	if limit > 0 {
		form.Set("limit", fmt.Sprintf("%d", limit))
	}
	env, err := c.do(ctx, http.MethodPost, "/api/v1/query_range", form)
	if err != nil {
		return nil, warnings(env), err
	}
	var data QueryData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, env.Warnings, err
	}
	return &data, env.Warnings, nil
}

// MetricNames returns all metric names via the __name__ label values endpoint.
func (c *Client) MetricNames(ctx context.Context) ([]string, error) {
	return c.LabelValues(ctx, "__name__")
}

// LabelNames returns all label names present in the store.
func (c *Client) LabelNames(ctx context.Context) ([]string, error) {
	env, err := c.do(ctx, http.MethodPost, "/api/v1/labels", url.Values{})
	if err != nil {
		return nil, err
	}
	return decodeStrings(env.Data)
}

// LabelValues returns the values for a single label name. The label name is
// percent-encoded into the path (e.g. "@resource.service.name" -> %40...).
func (c *Client) LabelValues(ctx context.Context, label string) ([]string, error) {
	path := "/api/v1/label/" + url.PathEscape(label) + "/values"
	env, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeStrings(env.Data)
}

// Series returns the label sets of series matching the selector over a time range.
func (c *Client) Series(ctx context.Context, match []string, start, end time.Time, limit int) ([]map[string]string, error) {
	form := url.Values{
		"start": {fmt.Sprintf("%d", start.Unix())},
		"end":   {fmt.Sprintf("%d", end.Unix())},
	}
	for _, m := range match {
		form.Add("match[]", m)
	}
	if limit > 0 {
		form.Set("limit", fmt.Sprintf("%d", limit))
	}
	env, err := c.do(ctx, http.MethodPost, "/api/v1/series", form)
	if err != nil {
		return nil, err
	}
	var out []map[string]string
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeStrings(data json.RawMessage) ([]string, error) {
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func warnings(env *Envelope) []string {
	if env == nil {
		return nil
	}
	return env.Warnings
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// CredentialsError wraps credential retrieval / auth failures so the TUI can
// show an actionable message instead of a raw 403.
type CredentialsError struct{ err error }

func (e *CredentialsError) Error() string {
	return "AWS credentials missing or expired: " + e.err.Error()
}
func (e *CredentialsError) Unwrap() error { return e.err }
