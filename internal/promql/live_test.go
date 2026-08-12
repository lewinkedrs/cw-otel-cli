package promql_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

// TestLive exercises the signing + endpoint path against a real account.
// Run with: CWPROMQL_LIVE=1 go test ./internal/promql -run TestLive -v
func TestLive(t *testing.T) {
	if os.Getenv("CWPROMQL_LIVE") == "" {
		t.Skip("set CWPROMQL_LIVE=1 to run the live smoke test")
	}
	region := "us-east-2"
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	c := promql.NewClient(region, cfg.Credentials)

	names, err := c.MetricNames(ctx)
	if err != nil {
		t.Fatalf("MetricNames: %v", err)
	}
	t.Logf("metric names: %d", len(names))
	if len(names) == 0 {
		t.Fatal("expected at least one metric name")
	}

	vec, warns, err := c.Query(ctx, `{"up"}`, time.Time{}, 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	samples, err := vec.DecodeVector()
	if err != nil {
		t.Fatalf("DecodeVector: %v", err)
	}
	t.Logf("instant vector series: %d, warnings=%v", len(samples), warns)

	end := time.Now()
	mat, _, err := c.QueryRange(ctx, `sum({"up"})`, end.Add(-30*time.Minute), end, 60*time.Second, 500)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	series, err := mat.DecodeMatrix()
	if err != nil {
		t.Fatalf("DecodeMatrix: %v", err)
	}
	if len(series) == 0 {
		t.Fatal("expected matrix data")
	}
	t.Logf("range matrix series: %d, points: %d", len(series), len(series[0].Values))
}
