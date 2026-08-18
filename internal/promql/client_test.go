package promql

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// stubCreds is a minimal static credentials provider for tests.
type stubCreds struct{}

func (stubCreds) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: "AKIDTEST", SecretAccessKey: "SECRETTEST"}, nil
}

// testClient points a real Client at an httptest server URL.
func testClient(url string) *Client {
	c := NewClient("us-east-2", stubCreds{})
	c.baseURL = url
	return c
}

func TestClientSignsAndParsesInstantQuery(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"__name__":"up"},"value":[1700000000,"1"]}]}}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	data, warns, err := c.Query(context.Background(), `{"up"}`, time.Time{}, 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256") {
		t.Errorf("request not SigV4-signed; Authorization=%q", gotAuth)
	}
	if gotPath != "/api/v1/query" {
		t.Errorf("path = %q, want /api/v1/query", gotPath)
	}
	if !strings.Contains(gotBody, "query=") || !strings.Contains(gotBody, "limit=5") {
		t.Errorf("body missing query/limit: %q", gotBody)
	}
	vec, err := data.DecodeVector()
	if err != nil {
		t.Fatalf("DecodeVector: %v", err)
	}
	if len(vec) != 1 || vec[0].Metric["__name__"] != "up" || vec[0].Value.Value != "1" {
		t.Errorf("unexpected vector: %+v", vec)
	}
}

func TestClientRangeQueryMatrixAndWarnings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"success","warnings":["truncated"],"data":{"resultType":"matrix","result":[
			{"metric":{},"values":[[1700000000,"10"],[1700000060,"12"]]}]}}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	end := time.Now()
	data, warns, err := c.QueryRange(context.Background(), `sum({"up"})`, end.Add(-time.Hour), end, time.Minute, 500)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(warns) != 1 || warns[0] != "truncated" {
		t.Errorf("warnings = %v, want [truncated]", warns)
	}
	m, err := data.DecodeMatrix()
	if err != nil {
		t.Fatalf("DecodeMatrix: %v", err)
	}
	if len(m) != 1 || len(m[0].Values) != 2 {
		t.Fatalf("unexpected matrix: %+v", m)
	}
	if v, _ := m[0].Values[1].ParsedValue(); v != 12 {
		t.Errorf("second sample = %v, want 12", v)
	}
}

func TestClientErrorEnvelopeMapsToAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"Selector must have a metric name"}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, _, err := c.Query(context.Background(), `{__name__=~".+"}`, time.Time{}, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.ErrorType != "bad_data" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}

func TestClientLabelValuesAndSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/label/"):
			_, _ = w.Write([]byte(`{"status":"success","data":["cart","checkout"]}`))
		case r.URL.Path == "/api/v1/series":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"__name__":"up","pod":"a"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	vals, err := c.LabelValues(context.Background(), "@resource.service.name")
	if err != nil {
		t.Fatalf("LabelValues: %v", err)
	}
	if len(vals) != 2 || vals[0] != "cart" {
		t.Errorf("label values = %v", vals)
	}
	end := time.Now()
	sets, err := c.Series(context.Background(), []string{`{"up"}`}, end.Add(-time.Hour), end, 0)
	if err != nil {
		t.Fatalf("Series: %v", err)
	}
	if len(sets) != 1 || sets[0]["pod"] != "a" {
		t.Errorf("series = %v", sets)
	}
}
