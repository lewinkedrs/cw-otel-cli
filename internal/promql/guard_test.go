package promql

import "testing"

func TestHasMetricName(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{`{"up"}`, true},
		{`{up}`, true},
		{`{__name__="up"}`, true},
		{`sum by ("@resource.service.name")({"http.server.duration_count"})`, true},
		{`{"http.server.active_requests","@resource.service.name"="myservice"}`, true},
		{`1+1`, true}, // scalar, no braces -> let server decide
		{``, false},
		{`{__name__=~".+"}`, false}, // regex on name only
		{`{"@resource.service.name"="cart"}`, false}, // label-only selector, no metric name
	}
	for _, c := range cases {
		if got := HasMetricName(c.expr); got != c.want {
			t.Errorf("HasMetricName(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}
