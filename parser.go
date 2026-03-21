package ghsummon

import (
	"regexp"
	"strings"
)

// Prompt represents a single @copilot prompt found in a file.
type Prompt struct {
	FilePath  string // File path where the prompt was found
	StartLine int    // 1-based line number where @copilot appears
	EndLine   int    // 1-based line number of the last continuation line
	Text      string // Combined prompt text (continuation lines joined with newlines, leading indent stripped)
}

var copilotPattern = regexp.MustCompile(`^@copilot (.+)$`)

// ParsePrompts parses file content and returns all @copilot prompts found.
// filePath is used to populate Prompt.FilePath.
func ParsePrompts(filePath string, content string) []Prompt {
	lines := strings.Split(content, "\n")
	var prompts []Prompt

	for i := 0; i < len(lines); i++ {
		m := copilotPattern.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		startLine := i + 1
		textParts := []string{m[1]}
		endLine := startLine

		for i+1 < len(lines) {
			next := lines[i+1]
			stripped, ok := stripIndent(next)
			if !ok {
				break
			}
			textParts = append(textParts, stripped)
			i++
			endLine = i + 1
		}

		prompts = append(prompts, Prompt{
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
			Text:      strings.Join(textParts, "\n"),
		})
	}
	return prompts
}

// stripIndent checks if a line is a continuation line (starts with tab or 2+ spaces).
// It returns the line with the leading indent stripped and true, or ("", false) if not a continuation.
func stripIndent(line string) (string, bool) {
	if line == "" {
		return "", false
	}
	if line[0] == '\t' {
		return line[1:], true
	}
	if len(line) >= 2 && line[0] == ' ' && line[1] == ' ' {
		// Strip leading spaces (the contiguous block of leading spaces)
		return strings.TrimLeft(line, " "), true
	}
	return "", false
}
