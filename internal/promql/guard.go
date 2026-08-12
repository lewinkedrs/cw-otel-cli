package promql

import (
	"regexp"
	"strings"
)

// The CloudWatch PromQL API rejects any selector that lacks an *exact* metric
// name matcher (HTTP 400: "Selector must have a metric name"). This includes
// bare-braces label-only selectors and regex-on-__name__ selectors. We do a
// lightweight structural check to warn the user before the request is sent.

// bareNameRe matches a leading quoted or unquoted metric name inside braces,
// e.g. {"http.server.duration"} or {up} or {__name__="up"}.
var (
	// A bare metric name is a quoted token immediately followed by ',' or '}'
	// (i.e. NOT "key"="val", which is a label matcher).
	quotedFirstRe  = regexp.MustCompile(`\{\s*"([^"]+)"\s*[,}]`)
	nameEqRe       = regexp.MustCompile(`__name__\s*=\s*"[^"]+"`)
	nameRegexRe    = regexp.MustCompile(`__name__\s*=~`)
	bareIdentFirst = regexp.MustCompile(`\{\s*([A-Za-z_][A-Za-z0-9_.:]*)\s*[},]`)
)

// HasMetricName reports whether expr contains at least one exact metric-name
// matcher, which the CloudWatch PromQL API requires. It is heuristic, not a
// full PromQL parser: it errs toward permitting queries it cannot classify.
func HasMetricName(expr string) bool {
	e := expr
	if strings.TrimSpace(e) == "" {
		return false
	}
	// __name__=~ is a regex, which the API rejects as a name matcher — but the
	// selector may still carry a valid quoted/bare exact name elsewhere.
	switch {
	case quotedFirstRe.MatchString(e):
		return true
	case nameEqRe.MatchString(e):
		return true
	case bareIdentFirst.MatchString(e):
		return true
	}
	// Regex-only on __name__ with nothing else -> not a valid exact matcher.
	if nameRegexRe.MatchString(e) {
		return false
	}
	// If there are no braces at all (e.g. a scalar expression like "1+1"),
	// let the server decide rather than blocking.
	return !strings.Contains(e, "{")
}
