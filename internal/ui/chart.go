package ui

import "strings"

var shadeChars = []rune{'░', '▒', '▓', '█'}

// renderAreaChart renders a gradient area chart of the given values using shade
// characters. The bottom rows are most solid (█) and the top rows least solid
// (░), so the area under the curve fades upward. Values are normalized to the
// maximum of the displayed window. The most recent value is placed at the
// right edge; empty columns are left-padded with spaces.
func renderAreaChart(values []float64, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	n := len(values)
	if n == 0 {
		return strings.Repeat("\n", height)
	}
	start := n - width
	if start < 0 {
		start = 0
	}
	samples := values[start:]
	offset := width - len(samples)

	maxV := 0.0
	for _, v := range samples {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		maxV = 1
	}

	rows := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		shade := shadeFor(float64(r+1) / float64(height))
		for x := 0; x < width; x++ {
			if x < offset {
				b.WriteByte(' ')
				continue
			}
			filled := samples[x-offset] / maxV * float64(height)
			if float64(height-r) <= filled {
				b.WriteRune(shade)
			} else {
				b.WriteByte(' ')
			}
		}
		rows[r] = b.String()
	}
	return strings.Join(rows, "\n")
}

func shadeFor(solidity float64) rune {
	switch {
	case solidity >= 0.75:
		return shadeChars[3]
	case solidity >= 0.5:
		return shadeChars[2]
	case solidity >= 0.25:
		return shadeChars[1]
	default:
		return shadeChars[0]
	}
}
