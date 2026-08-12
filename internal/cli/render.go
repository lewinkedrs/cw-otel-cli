package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/guptarohit/asciigraph"

	"github.com/lewinkedrs/cw-otel-cli/internal/promql"
)

// identifyingLabels are preferred, in order, for building a short series name.
var identifyingLabels = []string{
	"@resource.service.name",
	"@resource.k8s.pod.name",
	"@resource.k8s.container.name",
	"@resource.k8s.namespace.name",
	"InstanceId",
	"@resource.cloud.resource_id",
}

func shortName(metric map[string]string) string {
	if len(metric) == 0 {
		return "" // unlabeled (e.g. an aggregation); caller supplies a fallback
	}
	name := metric["__name__"]
	for _, k := range identifyingLabels {
		if v, ok := metric[k]; ok {
			if name != "" {
				return fmt.Sprintf("%s{%s=%s}", name, trimScope(k), v)
			}
			return trimScope(k) + "=" + v
		}
	}
	if name != "" {
		return name
	}
	keys := make([]string, 0, len(metric))
	for k := range metric {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+metric[k])
	}
	return strings.Join(parts, ",")
}

func trimScope(label string) string {
	for _, p := range []string{"@resource.", "@instrumentation.", "@aws.", "@datapoint."} {
		label = strings.TrimPrefix(label, p)
	}
	return label
}

// nameOr returns the series' short name, or fallback when it has no labels.
func nameOr(metric map[string]string, fallback string) string {
	if n := shortName(metric); n != "" {
		return n
	}
	return fallback
}

// ---- instant (vector) ----

func printVectorTable(w io.Writer, samples []promql.VectorSample, fallback string) {
	rows := make([][2]string, 0, len(samples))
	for _, s := range samples {
		rows = append(rows, [2]string{nameOr(s.Metric, fallback), s.Value.Value})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SERIES\tVALUE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d series\n", len(rows))
}

func printVectorCSV(w io.Writer, samples []promql.VectorSample, fallback string) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"series", "value", "timestamp"})
	for _, s := range samples {
		if err := cw.Write([]string{nameOr(s.Metric, fallback), s.Value.Value, fmt.Sprintf("%.0f", s.Value.Time)}); err != nil {
			return err
		}
	}
	return cw.Error()
}

// ---- range (matrix) ----

var chartPalette = []asciigraph.AnsiColor{
	asciigraph.Green, asciigraph.SkyBlue, asciigraph.Yellow, asciigraph.Magenta,
	asciigraph.Orange, asciigraph.Cyan, asciigraph.Red, asciigraph.Pink,
}

const maxChartSeries = 6

// renderMatrixChart plots up to maxChartSeries series as a colored line chart.
func renderMatrixChart(series []promql.MatrixSample, width, height int, color bool, fallback string) string {
	if len(series) == 0 {
		return "no data in range"
	}
	// Sort for stable color assignment.
	sort.Slice(series, func(i, j int) bool { return nameOr(series[i].Metric, fallback) < nameOr(series[j].Metric, fallback) })

	shown := series
	extra := 0
	if len(shown) > maxChartSeries {
		extra = len(shown) - maxChartSeries
		shown = shown[:maxChartSeries]
	}

	data := make([][]float64, 0, len(shown))
	legends := make([]string, 0, len(shown))
	colors := make([]asciigraph.AnsiColor, 0, len(shown))
	for i, s := range shown {
		vals := floatValues(s.Values)
		if len(vals) == 0 {
			continue
		}
		data = append(data, vals)
		legends = append(legends, nameOr(s.Metric, fallback))
		colors = append(colors, chartPalette[i%len(chartPalette)])
	}
	if len(data) == 0 {
		return "no numeric samples in range"
	}

	if width < 20 {
		width = 20
	}
	if height < 4 {
		height = 4
	}
	opts := []asciigraph.Option{
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Precision(2),
		asciigraph.SeriesLegends(legends...),
	}
	if color {
		opts = append(opts, asciigraph.SeriesColors(colors...))
	}
	out := asciigraph.PlotMany(data, opts...)
	if extra > 0 {
		out += fmt.Sprintf("\n(+%d more series not shown; narrow the query or use -o table)", extra)
	}
	return out
}

// printMatrixTable prints one row per series with a sparkline and summary stats.
func printMatrixTable(w io.Writer, series []promql.MatrixSample, fallback string) {
	sort.Slice(series, func(i, j int) bool { return nameOr(series[i].Metric, fallback) < nameOr(series[j].Metric, fallback) })
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SERIES\tTREND\tMIN\tMAX\tLAST")
	for _, s := range series {
		vals := floatValues(s.Values)
		if len(vals) == 0 {
			continue
		}
		mn, mx, last := stats(vals)
		fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\n", nameOr(s.Metric, fallback), sparkline(vals, sparkWidth), mn, mx, last)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d series\n", len(series))
}

// ---- shared helpers ----

func floatValues(samples []promql.Sample) []float64 {
	out := make([]float64, 0, len(samples))
	for _, s := range samples {
		if f, err := s.ParsedValue(); err == nil {
			out = append(out, f)
		}
	}
	return out
}

func stats(vals []float64) (min, max, last float64) {
	min, max = math.Inf(1), math.Inf(-1)
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max, vals[len(vals)-1]
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

const sparkWidth = 50

// sparkline renders vals as unicode bars, downsampling (bucket-averaging) to at
// most maxWidth columns so wide series stay on one line.
func sparkline(vals []float64, maxWidth int) string {
	if len(vals) == 0 {
		return ""
	}
	vals = downsample(vals, maxWidth)
	min, max, _ := stats(vals)
	rng := max - min
	var b strings.Builder
	for _, v := range vals {
		idx := 0
		if rng > 0 {
			idx = int(math.Round((v - min) / rng * float64(len(sparkRunes)-1)))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

// downsample bucket-averages vals down to at most n points.
func downsample(vals []float64, n int) []float64 {
	if n <= 0 || len(vals) <= n {
		return vals
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		lo := i * len(vals) / n
		hi := (i + 1) * len(vals) / n
		if hi <= lo {
			hi = lo + 1
		}
		var sum float64
		for _, v := range vals[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printStrings(w io.Writer, ss []string, asJSON bool) error {
	if asJSON {
		return printJSON(w, ss)
	}
	for _, s := range ss {
		fmt.Fprintln(w, s)
	}
	return nil
}
