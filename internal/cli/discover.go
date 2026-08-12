package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var flagFilter string

var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   "List metric names (__name__ values)",
	Example: "  cwpromql metrics --filter http",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		c, err := newClient(ctx)
		if err != nil {
			return err
		}
		names, err := c.MetricNames(ctx)
		if err != nil {
			return err
		}
		if flagFilter != "" {
			names = filterContains(names, flagFilter)
		}
		return printStrings(os.Stdout, names, flagOutput == "json")
	},
}

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "List all label names present in the metric store",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		c, err := newClient(ctx)
		if err != nil {
			return err
		}
		names, err := c.LabelNames(ctx)
		if err != nil {
			return err
		}
		if flagFilter != "" {
			names = filterContains(names, flagFilter)
		}
		return printStrings(os.Stdout, names, flagOutput == "json")
	},
}

var labelValuesCmd = &cobra.Command{
	Use:     "label-values <label>",
	Short:   "List the values of a single label",
	Args:    cobra.ExactArgs(1),
	Example: "  cwpromql label-values @resource.service.name",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		c, err := newClient(ctx)
		if err != nil {
			return err
		}
		vals, err := c.LabelValues(ctx, args[0])
		if err != nil {
			return err
		}
		if flagFilter != "" {
			vals = filterContains(vals, flagFilter)
		}
		return printStrings(os.Stdout, vals, flagOutput == "json")
	},
}

var seriesCmd = &cobra.Command{
	Use:     "series <selector>",
	Short:   "List series label sets matching a selector",
	Args:    cobra.ExactArgs(1),
	Example: `  cwpromql series '{"up"}' --since 1h`,
	RunE: func(cmd *cobra.Command, args []string) error {
		since, err := time.ParseDuration(flagSeriesSince)
		if err != nil {
			return fmt.Errorf("--since: %w", err)
		}
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		c, err := newClient(ctx)
		if err != nil {
			return err
		}
		end := time.Now()
		sets, err := c.Series(ctx, []string{args[0]}, end.Add(-since), end, limitOrDefault(0))
		if err != nil {
			return err
		}
		if flagOutput == "json" {
			return printJSON(os.Stdout, sets)
		}
		for _, s := range sets {
			fmt.Fprintln(os.Stdout, shortName(s))
		}
		fmt.Fprintf(os.Stdout, "\n%d series\n", len(sets))
		return nil
	},
}

var flagSeriesSince string

func filterContains(in []string, sub string) []string {
	sub = strings.ToLower(sub)
	out := in[:0]
	for _, s := range in {
		if strings.Contains(strings.ToLower(s), sub) {
			out = append(out, s)
		}
	}
	return out
}

func init() {
	metricsCmd.Flags().StringVar(&flagFilter, "filter", "", "case-insensitive substring filter")
	labelsCmd.Flags().StringVar(&flagFilter, "filter", "", "case-insensitive substring filter")
	labelValuesCmd.Flags().StringVar(&flagFilter, "filter", "", "case-insensitive substring filter")
	seriesCmd.Flags().StringVar(&flagSeriesSince, "since", "1h", "look-back window (e.g. 30m, 1h)")
}
