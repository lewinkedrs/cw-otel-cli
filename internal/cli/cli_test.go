package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

func TestResolveStep(t *testing.T) {
	// Explicit step is parsed as-is.
	got, err := resolveStep("30s", time.Hour)
	if err != nil || got != 30*time.Second {
		t.Fatalf("explicit step = %v, %v", got, err)
	}
	// Auto step ~ window/240 with a 15s floor.
	got, err = resolveStep("", time.Hour)
	if err != nil || got != 15*time.Second { // 3600/240 = 15s
		t.Errorf("auto step (1h) = %v", got)
	}
	got, _ = resolveStep("", 24*time.Hour)
	if got != 360*time.Second { // 86400/240 = 360s
		t.Errorf("auto step (24h) = %v", got)
	}
	// Tiny window still floors to 15s.
	got, _ = resolveStep("", time.Minute)
	if got != 15*time.Second {
		t.Errorf("auto step (1m) = %v, want 15s floor", got)
	}
	// Bad duration errors.
	if _, err := resolveStep("banana", time.Hour); err == nil {
		t.Error("expected error for bad step")
	}
}

func TestLimitOrDefault(t *testing.T) {
	flagLimit = 0
	if got := limitOrDefault(500); got != 500 {
		t.Errorf("limitOrDefault(500) with flag 0 = %d", got)
	}
	flagLimit = 42
	if got := limitOrDefault(500); got != 42 {
		t.Errorf("limitOrDefault(500) with flag 42 = %d", got)
	}
	flagLimit = 0 // reset for other tests
}

func TestFilterContains(t *testing.T) {
	in := []string{"http.server.duration", "kafka.lag", "HTTP.client.count"}
	got := filterContains(append([]string(nil), in...), "http")
	if len(got) != 2 {
		t.Fatalf("filter http = %v", got)
	}
	if got := filterContains(append([]string(nil), in...), "zzz"); len(got) != 0 {
		t.Errorf("filter zzz = %v", got)
	}
}

func TestPrintVectorCSV(t *testing.T) {
	samples := []promql.VectorSample{
		{Metric: map[string]string{"__name__": "up"}, Value: promql.Sample{Time: 1700000000, Value: "1"}},
		{Metric: map[string]string{}, Value: promql.Sample{Time: 1700000000, Value: "3"}},
	}
	var b strings.Builder
	if err := printVectorCSV(&b, samples, "sum({\"up\"})"); err != nil {
		t.Fatalf("printVectorCSV: %v", err)
	}
	out := b.String()
	if !strings.HasPrefix(out, "series,value,timestamp") {
		t.Errorf("missing header: %q", out)
	}
	if !strings.Contains(out, "up,1,1700000000") {
		t.Errorf("missing labeled row: %q", out)
	}
	// Empty-label series uses the fallback expression.
	if !strings.Contains(out, `sum({""up""})`) && !strings.Contains(out, "sum(") {
		t.Errorf("missing fallback row: %q", out)
	}
}
