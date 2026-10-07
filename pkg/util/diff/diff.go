// Package diff renders the change between line sequences in the standard diff style.
package diff

import (
	"slices"
	"strings"
)

// Lines renders oldLines→newLines as standard diff lines — '-' marks a removal, '+'
// an addition — with no context lines. Line order follows the longest common
// subsequence, so shared lines anchor their churn around them.
func Lines(oldLines, newLines []string) string {
	common := lcs(oldLines, newLines)
	var b strings.Builder
	var i, j int
	for _, line := range common {
		for oldLines[i] != line {
			b.WriteString("-" + oldLines[i] + "\n")
			i++
		}
		for newLines[j] != line {
			b.WriteString("+" + newLines[j] + "\n")
			j++
		}
		i++
		j++
	}
	for ; i < len(oldLines); i++ {
		b.WriteString("-" + oldLines[i] + "\n")
	}
	for ; j < len(newLines); j++ {
		b.WriteString("+" + newLines[j] + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// lcs returns the longest common subsequence of the two line slices
func lcs(a, b []string) []string {
	// lengths[i][j] counts the lcs of a[i:] and b[j:]
	lengths := make([][]int, len(a)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(b)+1)
	}
	for i, v := range slices.Backward(a) {
		for j := len(b) - 1; j >= 0; j-- {
			if v == b[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
			} else {
				lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
			}
		}
	}
	var out []string
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]: // the tie advances a: removals lead additions
			i++
		default:
			j++
		}
	}
	return out
}
