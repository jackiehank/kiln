package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jackiehank/kiln/internal/model"
)

// ordinalList renders an issue's actions the way a caller would address them:
// `1.ask 2.collect …`. It is what a failed lookup offers instead of a bare
// "not found", so the next attempt needs no second command.
func ordinalList(p *model.Issue) string {
	parts := make([]string, 0, len(p.Actions))
	for i, a := range p.Actions {
		parts = append(parts, fmt.Sprintf("%d.%s", i+1, a.Kind))
	}
	return strings.Join(parts, " ")
}

// matchingOrdinals lists the 1-based positions of one action kind, in order, so a
// hint can hand back addresses the caller can use verbatim.
func matchingOrdinals(p *model.Issue, kind string) []int {
	var out []int
	for i, a := range p.Actions {
		if a.Kind == kind {
			out = append(out, i+1)
		}
	}
	return out
}

// ordinalNumbers renders 1-based ordinals as `4, 5, 6`.
func ordinalNumbers(ns []int) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}
