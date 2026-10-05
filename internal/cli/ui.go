package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
)

type TerminalUI struct {
	out io.Writer
}

func NewUI() *TerminalUI {
	return &TerminalUI{out: os.Stdout}
}

func (ui *TerminalUI) Header(title string) {
	fmt.Fprintf(ui.out, "\n%s%s%s\n\n", ColorBold+ColorCyan, title, ColorReset)
}

func (ui *TerminalUI) Section(name string) {
	fmt.Fprintf(ui.out, "\n%s%s%s\n\n", ColorBold, name, ColorReset)
}

func (ui *TerminalUI) Step(name string, success bool, detail string) {
	icon := fmt.Sprintf("%s✓%s", ColorGreen, ColorReset)
	if !success {
		icon = fmt.Sprintf("%s✗%s", ColorRed, ColorReset)
	}

	if detail != "" {
		fmt.Fprintf(ui.out, "%s %-30s %s%s%s\n", icon, name, ColorGray, detail, ColorReset)
	} else {
		fmt.Fprintf(ui.out, "%s %s\n", icon, name)
	}
}

func (ui *TerminalUI) Warn(msg string) {
	fmt.Fprintf(ui.out, "%s! %s%s\n", ColorYellow, msg, ColorReset)
}

func (ui *TerminalUI) Error(msg string) {
	fmt.Fprintf(ui.out, "%s✗ Error: %s%s\n", ColorRed, msg, ColorReset)
}

func (ui *TerminalUI) Success(msg string) {
	fmt.Fprintf(ui.out, "%s✓ %s%s\n", ColorGreen, msg, ColorReset)
}

func (ui *TerminalUI) Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	for _, r := range rows {
		for i, cell := range r {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, h := range headers {
		fmt.Fprintf(ui.out, "%s%-*s%s  ", ColorBold, widths[i], h, ColorReset)
	}
	fmt.Fprintln(ui.out)

	// Print rows
	for _, r := range rows {
		for i, cell := range r {
			w := widths[i]
			fmt.Fprintf(ui.out, "%-*s  ", w, cell)
		}
		fmt.Fprintln(ui.out)
	}
}

func (ui *TerminalUI) KeyValue(key, val string) {
	fmt.Fprintf(ui.out, "%s%-16s%s %s\n", ColorGray, key+":", ColorReset, val)
}

func (ui *TerminalUI) MaskSecret(secret string) string {
	if len(secret) == 0 {
		return "********"
	}
	return "********"
}

func (ui *TerminalUI) Divider() {
	fmt.Fprintln(ui.out, strings.Repeat("─", 45))
}
