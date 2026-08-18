// Package cli implements the cwpromql command-line interface.
package cli

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/lewinkedrs/cw-otel-cli/internal/awscfg"
	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

// version is the build version. It defaults to "dev" and can be overridden at
// build time with -ldflags "-X .../internal/cli.version=v1.2.3". When unset,
// it falls back to the module version embedded by `go install`.
var version = "dev"

func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

// Persistent (global) flags.
var (
	flagRegion  string
	flagProfile string
	flagOutput  string
	flagLimit   int
	flagNoColor bool
)

const defaultTimeout = 30 * time.Second

var rootCmd = &cobra.Command{
	Use:   "cwpromql",
	Short: "Query CloudWatch OTLP metrics with PromQL from the terminal",
	Long: `cwpromql queries CloudWatch's Prometheus-compatible PromQL API
(SigV4-signed, service "monitoring") which has no aws-cli equivalent.

It renders instant queries as tables and range queries as ASCII charts,
and can emit JSON/CSV for scripting.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       versionString(),
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagRegion, "region", "us-east-2", "AWS region hosting the metrics")
	pf.StringVar(&flagProfile, "profile", "", "AWS shared-config profile (default credential chain if empty)")
	pf.StringVarP(&flagOutput, "output", "o", "", "output format: table|chart|json|csv (default depends on command)")
	pf.IntVar(&flagLimit, "limit", 0, "max series to return (0 = API default)")
	pf.BoolVar(&flagNoColor, "no-color", false, "disable colored chart output")

	rootCmd.AddCommand(queryCmd, rangeCmd, metricsCmd, labelsCmd, labelValuesCmd, seriesCmd)
}

// newClient builds a signed PromQL client from the global flags.
func newClient(ctx context.Context) (*promql.Client, error) {
	cfg, err := awscfg.Load(ctx, flagRegion, flagProfile)
	if err != nil {
		return nil, err
	}
	return promql.NewClient(flagRegion, cfg.Credentials), nil
}

// ctxWithTimeout returns a background context with the default timeout.
func ctxWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}
