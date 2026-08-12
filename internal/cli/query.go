package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

var queryCmd = &cobra.Command{
	Use:   "query <promql>",
	Short: "Run an instant PromQL query (single point in time)",
	Args:  cobra.ExactArgs(1),
	Example: `  cwpromql query '{"up"}'
  cwpromql query 'sum by ("@resource.service.name")({"up"})'
  cwpromql query '{"up"}' -o json | jq .`,
	RunE: func(cmd *cobra.Command, args []string) error {
		expr := args[0]
		if !promql.HasMetricName(expr) {
			return fmt.Errorf("selector needs an exact metric name, e.g. {\"up\"} or {__name__=\"up\"}; " +
				"CloudWatch rejects label-only or regex-name selectors")
		}
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		c, err := newClient(ctx)
		if err != nil {
			return err
		}
		data, warns, err := c.Query(ctx, expr, time.Time{}, flagLimit)
		if err != nil {
			return err
		}
		samples, err := data.DecodeVector()
		if err != nil {
			return err
		}
		switch flagOutput {
		case "json":
			if err := printJSON(os.Stdout, samples); err != nil {
				return err
			}
		case "csv":
			if err := printVectorCSV(os.Stdout, samples, expr); err != nil {
				return err
			}
		default: // table
			printVectorTable(os.Stdout, samples, expr)
		}
		printWarnings(warns)
		return nil
	},
}

func printWarnings(warns []string) {
	for _, w := range warns {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
}
