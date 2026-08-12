// Command cwpromql is a CLI for querying CloudWatch OTLP metrics via the
// Prometheus-compatible PromQL HTTP API (SigV4-signed, service "monitoring").
package main

import "github.com/lewinkedrs/cw-otel-cli/internal/cli"

func main() {
	cli.Execute()
}
