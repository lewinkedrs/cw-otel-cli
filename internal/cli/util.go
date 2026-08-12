package cli

import (
	"os"

	"golang.org/x/term"
)

// termWidth returns the terminal width, falling back to 100 columns when
// stdout is not a terminal (e.g. piped output).
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 100
}
