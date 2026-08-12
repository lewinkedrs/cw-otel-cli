package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

var (
	flagSince  string
	flagStep   string
	flagWatch  string
	flagHeight int
	flagWidth  int
)

var rangeCmd = &cobra.Command{
	Use:   "range <promql>",
	Short: "Run a range query and render it as an ASCII chart",
	Args:  cobra.ExactArgs(1),
	Example: `  cwpromql range 'sum({"up"})' --since 1h
  cwpromql range '{"up"}' --since 3h --step 60s
  cwpromql range 'sum({"up"})' --since 30m --watch 10s
  cwpromql range '{"up"}' --since 1h -o table`,
	RunE: func(cmd *cobra.Command, args []string) error {
		expr := args[0]
		if !promql.HasMetricName(expr) {
			return fmt.Errorf("selector needs an exact metric name, e.g. {\"up\"} or {__name__=\"up\"}; " +
				"CloudWatch rejects label-only or regex-name selectors")
		}
		since, err := time.ParseDuration(flagSince)
		if err != nil {
			return fmt.Errorf("--since: %w", err)
		}
		step, err := resolveStep(flagStep, since)
		if err != nil {
			return err
		}

		var watch time.Duration
		if flagWatch != "" {
			if watch, err = time.ParseDuration(flagWatch); err != nil {
				return fmt.Errorf("--watch: %w", err)
			}
		}

		run := func() error { return runRange(expr, since, step) }
		if watch <= 0 {
			return run()
		}
		// Watch mode: clear screen and re-render on an interval.
		for {
			fmt.Print("\033[H\033[2J") // home + clear
			fmt.Printf("cwpromql range %q  since=%s step=%s  every %s  (Ctrl+C to stop)\n\n",
				expr, since, step, watch)
			if err := run(); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			time.Sleep(watch)
		}
	},
}

func runRange(expr string, since, step time.Duration) error {
	ctx, cancel := ctxWithTimeout()
	defer cancel()
	c, err := newClient(ctx)
	if err != nil {
		return err
	}
	end := time.Now()
	start := end.Add(-since)
	data, warns, err := c.QueryRange(ctx, expr, start, end, step, limitOrDefault(500))
	if err != nil {
		return err
	}
	series, err := data.DecodeMatrix()
	if err != nil {
		return err
	}
	switch flagOutput {
	case "json":
		if err := printJSON(os.Stdout, series); err != nil {
			return err
		}
	case "table":
		printMatrixTable(os.Stdout, series, expr)
	default: // chart
		h, w := flagHeight, flagWidth
		if h == 0 {
			h = 12
		}
		if w == 0 {
			w = termWidth() - 14
		}
		fmt.Println(renderMatrixChart(series, w, h, !flagNoColor, expr))
	}
	printWarnings(warns)
	return nil
}

// resolveStep parses an explicit step, or auto-derives ~240 points across the
// window with a 15s floor.
func resolveStep(s string, since time.Duration) (time.Duration, error) {
	if s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("--step: %w", err)
		}
		return d, nil
	}
	step := since / 240
	if step < 15*time.Second {
		step = 15 * time.Second
	}
	return step.Round(time.Second), nil
}

func limitOrDefault(def int) int {
	if flagLimit > 0 {
		return flagLimit
	}
	return def
}

func init() {
	f := rangeCmd.Flags()
	f.StringVar(&flagSince, "since", "1h", "look-back window (e.g. 30m, 1h, 24h)")
	f.StringVar(&flagStep, "step", "", "resolution step (e.g. 60s); auto if unset")
	f.StringVar(&flagWatch, "watch", "", "re-render on an interval (e.g. 10s); Ctrl+C to stop")
	f.IntVar(&flagHeight, "height", 0, "chart height in rows (default 12)")
	f.IntVar(&flagWidth, "width", 0, "chart width in cols (default: terminal width)")
}
