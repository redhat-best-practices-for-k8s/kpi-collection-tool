package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"
	"golang.org/x/term"
)

type keyAction int

const (
	keyNone keyAction = iota
	keyLeft
	keyRight
	keyUp
	keyDown
	keyQuit
)

const (
	minViewportSamples = 2
	viewportStepDiv    = 10 // pan/zoom by 1/10 of visible window
	statusBarRows      = 1

	// ANSI escape sequences for terminal control.
	ansiClearScreen = "\033[2J"   // erase entire screen contents
	ansiCursorHome  = "\033[H"    // move cursor to row 1, column 1
	ansiCursorHide  = "\033[?25l" // make cursor invisible
	ansiCursorShow  = "\033[?25h" // restore cursor visibility
	// ansiCursorPos is a format string: move cursor to row %d, column 1.
	ansiCursorPos = "\033[%d;1H"

	// ANSI escape sequence bytes read from stdin in raw mode.
	ansiEsc        = 0x1b // starts an escape sequence
	ansiCSI        = '['  // Control Sequence Introducer (second byte)
	ansiArrowUp    = 'A'
	ansiArrowDown  = 'B'
	ansiArrowRight = 'C'
	ansiArrowLeft  = 'D'

	keyCtrlC = 3

	// Unicode arrow symbols for the status bar.
	arrowLeft  = "\u2190"
	arrowRight = "\u2192"
	arrowUp    = "\u2191"
	arrowDown  = "\u2193"
)

// clearScreen erases the terminal and moves the cursor to the top-left corner.
func clearScreen() {
	fmt.Print(ansiClearScreen + ansiCursorHome)
}

// hideCursor makes the terminal cursor invisible.
func hideCursor() {
	fmt.Print(ansiCursorHide)
}

// showCursor restores the terminal cursor visibility.
func showCursor() {
	fmt.Print(ansiCursorShow)
}

// moveCursorToRow positions the cursor at the beginning of the given row (1-based).
func moveCursorToRow(row int) {
	fmt.Printf(ansiCursorPos, row)
}

// viewport represents a sliding window over the full sample array.
// startIdx is the index of the first visible sample, count is how many
// samples are currently shown, and total is the overall dataset size.
// Pan shifts the window along the time axis; zoom grows or shrinks it
// symmetrically around its center.
type viewport struct {
	startIdx int
	count    int
	total    int
}

// panLeft shifts the visible window toward earlier samples by 10% of its
// current width. Stops at the beginning of the dataset.
func (v *viewport) panLeft() {
	step := max(v.count/viewportStepDiv, 1)
	v.startIdx = max(v.startIdx-step, 0)
}

// panRight shifts the visible window toward later samples by 10% of its
// current width. Stops at the end of the dataset.
func (v *viewport) panRight() {
	step := max(v.count/viewportStepDiv, 1)
	maxStart := v.total - v.count
	v.startIdx = min(v.startIdx+step, maxStart)
}

// zoomOut widens the window by 10%, expanding equally from both edges
// so the center of the view stays roughly the same. Capped at the full dataset.
func (v *viewport) zoomOut() {
	delta := max(v.count/viewportStepDiv, 1)
	newCount := min(v.count+delta, v.total)
	grown := newCount - v.count
	v.startIdx = max(v.startIdx-grown/2, 0)
	v.count = newCount
	v.clamp()
}

// zoomIn narrows the window by 10%, shrinking equally from both edges.
// Won't go below minViewportSamples (or total if the dataset is smaller).
func (v *viewport) zoomIn() {
	delta := max(v.count/viewportStepDiv, 1)
	minCount := min(minViewportSamples, v.total)
	newCount := max(v.count-delta, minCount)
	shrunk := v.count - newCount
	v.startIdx += shrunk / 2
	v.count = newCount
	v.clamp()
}

// clamp ensures the window stays within [0, total) after pan/zoom adjustments.
func (v *viewport) clamp() {
	if v.startIdx+v.count > v.total {
		v.startIdx = v.total - v.count
	}

	v.startIdx = max(v.startIdx, 0)
}

// RunInteractiveChart enters a full-screen interactive chart viewer
// with keyboard-driven pan (left/right) and zoom (up/down).
//
// The terminal is switched to raw mode so individual keypresses can be
// read without waiting for Enter. On exit (q or Ctrl+C) the original
// terminal state is always restored via defer, even on error.
func RunInteractiveChart(records []KPIRecord, kpiName string) error {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp < records[j].Timestamp
	})

	fd := int(os.Stdin.Fd())

	// Raw mode disables line buffering and echo, giving us byte-level
	// control over stdin reads for arrow-key detection.
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %w", err)
	}

	defer func() {
		_ = term.Restore(fd, oldState)
		showCursor()
	}()

	hideCursor()

	// Start with all samples visible; the user can zoom in from there.
	vp := viewport{startIdx: 0, count: len(records), total: len(records)}

	for {
		if err := renderFrame(records, kpiName, &vp); err != nil {
			return err
		}

		switch readKey(os.Stdin) {
		case keyQuit:
			clearScreen()
			return nil
		case keyLeft:
			vp.panLeft()
		case keyRight:
			vp.panRight()
		case keyUp:
			vp.zoomIn()
		case keyDown:
			vp.zoomOut()
		}
	}
}

// renderFrame redraws the entire screen: clears the terminal, renders the
// chart for the current viewport slice, and prints the status bar on the
// last row. Terminal size is queried on every frame so resizes are picked
// up on the next keypress.
func renderFrame(records []KPIRecord, kpiName string, vp *viewport) error {
	clearScreen()

	ttyWidth, ttyHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get terminal size: %w", err)
	}

	chartHeight := ttyHeight - statusBarRows

	slice := records[vp.startIdx : vp.startIdx+vp.count]
	values, times := extractValuesAndTimes(slice)

	opts := buildChartOptions(values, times, kpiName, ttyWidth, chartHeight)

	graph := asciigraph.Plot(values, opts...)
	// In raw mode \n only moves the cursor down without returning to
	// column 0, so we replace with \r\n for correct line rendering.
	fmt.Print(strings.ReplaceAll(graph, "\n", "\r\n"))

	printStatusBar(vp, ttyWidth, ttyHeight)

	return nil
}

func extractValuesAndTimes(records []KPIRecord) ([]float64, []time.Time) {
	values := make([]float64, len(records))
	times := make([]time.Time, len(records))

	for i, r := range records {
		values[i] = r.Value
		times[i], _ = time.Parse("2006-01-02 15:04:05", r.Timestamp)
	}

	return values, times
}

// printStatusBar renders the bottom-row legend showing the current sample
// range and available keyboard shortcuts. It is padded to the full terminal
// width so the entire row is overwritten (avoids leftover characters).
func printStatusBar(vp *viewport, width, height int) {
	endIdx := vp.startIdx + vp.count
	status := fmt.Sprintf(" Samples %d-%d of %d | %s/%s pan  %s/%s zoom  q quit",
		vp.startIdx+1, endIdx, vp.total, arrowLeft, arrowRight, arrowUp, arrowDown)

	moveCursorToRow(height)
	fmt.Printf("%-*s", width, status)
}

// readKey blocks until a single keypress is available and returns the
// corresponding action. Arrow keys are 3-byte ANSI escape sequences
// (ESC [ A/B/C/D); single-byte keys like 'q' and Ctrl+C are handled
// directly. Unrecognised keys return keyNone.
func readKey(r io.Reader) keyAction {
	var buf [1]byte
	if _, err := r.Read(buf[:]); err != nil {
		return keyQuit
	}

	switch buf[0] {
	case 'q', 'Q':
		return keyQuit
	case keyCtrlC:
		return keyQuit
	case ansiEsc:
		return readEscapeSeq(r)
	default:
		return keyNone
	}
}

// readEscapeSeq reads the two bytes following an ESC (0x1b) to identify
// an arrow key. Expected sequence: ESC '[' <direction-letter>.
func readEscapeSeq(r io.Reader) keyAction {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return keyNone
	}

	if buf[0] != ansiCSI {
		return keyNone
	}

	switch buf[1] {
	case ansiArrowUp:
		return keyUp
	case ansiArrowDown:
		return keyDown
	case ansiArrowRight:
		return keyRight
	case ansiArrowLeft:
		return keyLeft
	default:
		return keyNone
	}
}
