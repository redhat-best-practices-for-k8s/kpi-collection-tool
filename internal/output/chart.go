package output

import (
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/guptarohit/asciigraph"
	"golang.org/x/term"
)

const (
	// DefaultChartHeight is the fallback chart height (rows) when no TTY is available.
	DefaultChartHeight = 25
	// DefaultChartWidth is the fallback chart width (columns) when no TTY is available.
	DefaultChartWidth = 80
	// MinChartWidth is the minimum allowed value for --chart-width.
	MinChartWidth = 80
	// MinChartHeight is the minimum allowed value for --chart-height.
	MinChartHeight = 25
	// MaxChartDimension is the maximum allowed value for --chart-width and --chart-height.
	MaxChartDimension = 250

	defaultXAxisTickCount = 8
	yAxisDefaultOffset    = 3
	// extraRows accounts for the lines rendered outside the plot area:
	// X-axis separator (1) + X-axis labels (1) + caption (1)
	extraRows = 4
	// minYAxisLabelWidth is the minimum width of the Y-axis labels.
	minYAxisLabelWidth = 4
	minXAxisTickCount  = 2
	// yLabelFormatThreshold is the minimum absolute value above which label
	// suffixes (K, M, G, T) are applied to Y-axis labels.
	yLabelFormatThreshold = 10000
	yLabelFormatPrecision = 3
)

// yLabelScale pairs a power-of-ten divisor with its label suffix (e.g. 1e3 → "K").
// Used to keep Y-axis labels compact when values span large magnitudes.
type yLabelScale struct {
	divisor float64
	suffix  string
}

// yLabelScales lists the supported label suffixes from largest to smallest.
// chooseYLabelScale walks this list and picks the first scale whose divisor
// does not exceed the data's peak value.
var yLabelScales = []yLabelScale{
	{1e12, "T"},
	{1e9, "G"},
	{1e6, "M"},
	{1e3, "K"},
}

// chooseYLabelScale picks a consistent label scale for the Y-axis based on the
// largest absolute value among min and max. Returns divisor=0 when no
// scaling is needed.
func chooseYLabelScale(minVal, maxVal float64) yLabelScale {
	peak := max(math.Abs(minVal), math.Abs(maxVal))
	if peak < yLabelFormatThreshold {
		return yLabelScale{0, ""}
	}

	for _, scale := range yLabelScales {
		if peak >= scale.divisor {
			return scale
		}
	}

	return yLabelScale{0, ""}
}

// formatYLabel renders a float using the chosen Y-label scale. When no scaling is
// needed (divisor==0) the value is printed as-is with fixed precision.
func formatYLabel(v float64, scale yLabelScale) string {
	if scale.divisor == 0 {
		return fmt.Sprintf("%.*f", yLabelFormatPrecision, v)
	}

	return fmt.Sprintf("%.*f%s", yLabelFormatPrecision, v/scale.divisor, scale.suffix)
}

// PrintChart renders an ASCII line chart of KPI metric values over time.
// The X-axis uses the sample timestamp (Prometheus metric timestamp),
// widthOverride and heightOverride are user-specified total dimensions (0 means use default).
func PrintChart(records []KPIRecord, kpiName string, widthOverride, heightOverride int) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp < records[j].Timestamp
	})

	values := make([]float64, len(records))
	times := make([]time.Time, len(records))

	for i, r := range records {
		values[i] = r.Value
		times[i] = time.Unix(int64(r.Timestamp), 0)
	}

	opts := buildChartOptions(values, times, kpiName, widthOverride, heightOverride)

	graph := asciigraph.Plot(values, opts...)

	fmt.Println(graph)
	fmt.Printf("\nData points: %d\n", len(records))
}

// buildChartOptions computes the plot dimensions and returns the asciigraph
// option set needed to render a chart. It resolves the total canvas size
// (respecting TTY limits and user overrides), subtracts space for axes and
// the caption, chooses label scaling for Y labels, and picks a compact time
// format for X-axis ticks.
func buildChartOptions(
	values []float64, times []time.Time, kpiName string,
	widthOverride, heightOverride int,
) []asciigraph.Option {
	first := times[0]
	last := times[len(times)-1]

	firstUnix := float64(first.Unix())
	lastUnix := float64(last.Unix())

	tickCount := min(len(times), defaultXAxisTickCount)
	tickCount = max(tickCount, minXAxisTickCount)

	timeFmt := pickTimeFormat(first, last)

	totalWidth, totalHeight := resolveChartDimensions(widthOverride, heightOverride)

	minVal, maxVal := findMinAndMax(values)
	scale := chooseYLabelScale(minVal, maxVal)
	yLabelWidth := yAxisLabelWidth(values, scale)

	labelOverhang := (len(timeFmt) + 1) / 2
	plotWidth := totalWidth - yLabelWidth - yAxisDefaultOffset - labelOverhang

	plotHeight := totalHeight - extraRows

	return []asciigraph.Option{
		asciigraph.Height(plotHeight),
		asciigraph.Width(plotWidth),
		asciigraph.Caption(kpiName),
		asciigraph.XAxisRange(firstUnix, lastUnix),
		asciigraph.XAxisTickCount(tickCount),
		asciigraph.XAxisValueFormatter(func(v float64) string {
			return time.Unix(int64(v), 0).Format(timeFmt)
		}),
		asciigraph.YAxisValueFormatter(func(v float64) string {
			return formatYLabel(v, scale)
		}),
	}
}

// yAxisLabelWidth estimates the character width of the Y-axis labels based
// on the min/max data values formatted through the label formatter.
func yAxisLabelWidth(values []float64, scale yLabelScale) int {
	minVal, maxVal := findMinAndMax(values)
	w := max(len(formatYLabel(minVal, scale)), len(formatYLabel(maxVal, scale)))

	return max(w, minYAxisLabelWidth)
}

// findMinAndMax returns the smallest and largest values in the slice.
func findMinAndMax(values []float64) (float64, float64) {
	minVal, maxVal := values[0], values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}

		if v > maxVal {
			maxVal = v
		}
	}

	return minVal, maxVal
}

// resolveChartDimensions returns the total chart width and height.
// widthOverride/heightOverride of 0 mean "use default".
//
// TTY: uses terminal size by default; overrides replace them.
// Non-TTY: uses defaultChartWidth/defaultChartHeight; overrides replace them.
func resolveChartDimensions(widthOverride, heightOverride int) (int, int) {
	fd := int(os.Stdout.Fd())

	if !term.IsTerminal(fd) {
		return nonTTYDimensions(widthOverride, heightOverride)
	}

	ttyWidth, ttyHeight, err := term.GetSize(fd)
	if err != nil {
		return nonTTYDimensions(widthOverride, heightOverride)
	}

	return ttyDimensions(ttyWidth, ttyHeight, widthOverride, heightOverride)
}

// nonTTYDimensions returns chart dimensions for piped/redirected output
// where the terminal size is unknown. Falls back to compiled-in defaults
// unless the user provided explicit overrides.
func nonTTYDimensions(widthOverride, heightOverride int) (int, int) {
	w := DefaultChartWidth
	if widthOverride > 0 {
		w = widthOverride
	}

	h := DefaultChartHeight
	if heightOverride > 0 {
		h = heightOverride
	}

	return w, h
}

// ttyDimensions returns chart dimensions for interactive terminal output.
// By default the chart fills the terminal; overrides replace the defaults.
// Callers are expected to validate that overrides fit the terminal before
// reaching this point (see validateChartDimensionsFitTerminal).
func ttyDimensions(ttyWidth, ttyHeight, widthOverride, heightOverride int) (int, int) {
	width := ttyWidth
	height := max(ttyHeight, DefaultChartHeight)

	if widthOverride > 0 {
		width = widthOverride
	}

	if heightOverride > 0 {
		height = heightOverride
	}

	return width, height
}

// pickTimeFormat chooses a time format string based on the span between
// the first and last data points so tick labels stay compact.
func pickTimeFormat(first, last time.Time) string {
	span := last.Sub(first)

	switch {
	case span < time.Hour:
		return "15:04:05"
	case span < 24*time.Hour:
		return "15:04"
	default:
		return "Jan 02 15:04"
	}
}
