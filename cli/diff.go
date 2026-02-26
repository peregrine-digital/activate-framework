package main

import (
	"fmt"
	"strings"
)

// unifiedDiff produces a simple line-by-line unified diff using LCS.
func unifiedDiff(a, b, labelA, labelB string) string {
	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")

	if strings.Join(linesA, "\n") == strings.Join(linesB, "\n") {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- %s\n", labelA))
	out.WriteString(fmt.Sprintf("+++ %s\n", labelB))

	n, m := len(linesA), len(linesB)

	// LCS-based diff
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if linesA[i] == linesB[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && linesA[i] == linesB[j] {
			out.WriteString(fmt.Sprintf(" %s\n", linesA[i]))
			i++
			j++
		} else if j < m && (i >= n || dp[i][j+1] >= dp[i+1][j]) {
			out.WriteString(fmt.Sprintf("+%s\n", linesB[j]))
			j++
		} else if i < n {
			out.WriteString(fmt.Sprintf("-%s\n", linesA[i]))
			i++
		}
	}

	return out.String()
}
