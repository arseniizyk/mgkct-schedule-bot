package handlers

import "strings"

func formatCandidates(candidates []string) string {
	var sb strings.Builder
	for _, c := range candidates {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("- " + c)
	}
	return sb.String()
}
