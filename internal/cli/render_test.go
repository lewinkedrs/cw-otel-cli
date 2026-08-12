package cli

import "testing"

func TestShortNameAndFallback(t *testing.T) {
	if got := shortName(map[string]string{}); got != "" {
		t.Errorf("empty metric: got %q, want empty", got)
	}
	if got := nameOr(map[string]string{}, "sum(x)"); got != "sum(x)" {
		t.Errorf("nameOr fallback: got %q, want sum(x)", got)
	}
	m := map[string]string{"__name__": "up", "@resource.service.name": "cart"}
	if got := shortName(m); got != "up{service.name=cart}" {
		t.Errorf("labeled: got %q", got)
	}
}

func TestDownsample(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	out := downsample(in, 5)
	if len(out) != 5 {
		t.Fatalf("len = %d, want 5", len(out))
	}
	// First bucket averages {1,2} = 1.5, last averages {9,10} = 9.5.
	if out[0] != 1.5 || out[4] != 9.5 {
		t.Errorf("buckets = %v, want first 1.5 last 9.5", out)
	}
	// No-op when already small enough.
	if got := downsample(in, 100); len(got) != len(in) {
		t.Errorf("no-op downsample changed length: %d", len(got))
	}
}

func TestSparklineWidth(t *testing.T) {
	vals := make([]float64, 240)
	for i := range vals {
		vals[i] = float64(i)
	}
	s := []rune(sparkline(vals, 50))
	if len(s) != 50 {
		t.Errorf("sparkline width = %d, want 50", len(s))
	}
}
