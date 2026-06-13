package tools

import (
	"bufio"
	"fmt"
	"strings"

	"helpassist/internal/ui"
)

// Confirmer asks the user to approve a side-effecting action.
// AutoApprove short-circuits to true (used by the --yes flag).
type Confirmer struct {
	in          *bufio.Reader
	AutoApprove bool
}

// NewConfirmer wires a confirmer to a shared stdin reader so prompts and the
// REPL never fight over the same input stream.
func NewConfirmer(in *bufio.Reader, autoApprove bool) *Confirmer {
	return &Confirmer{in: in, AutoApprove: autoApprove}
}

// Ask shows a title, an optional multi-line preview, and waits for y/n.
// Returns true when the user approves.
func (c *Confirmer) Ask(title, preview string) bool {
	if c.AutoApprove {
		return true
	}
	fmt.Println()
	fmt.Println("  " + ui.Yellow(title))
	if preview != "" {
		for _, ln := range strings.Split(strings.TrimRight(preview, "\n"), "\n") {
			fmt.Println("  " + ui.Gray("│ ") + ln)
		}
	}
	fmt.Print("  " + ui.Bold("Разрешить? ") + ui.Gray("[y/N] "))
	line, err := c.in.ReadString('\n')
	if err != nil {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes" || ans == "д" || ans == "да"
}
