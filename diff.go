package ghsummon

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// emptyTreeHash is the SHA of git's empty tree, used for diffing the initial commit.
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf899d15f13a88e14"

// detectShallowAndDeepen checks if the repo is a shallow clone and deepens it if needed.
func detectShallowAndDeepen(ctx context.Context) error {
	if _, err := os.Stat(".git/shallow"); err != nil {
		return nil // not shallow
	}
	cmd := exec.CommandContext(ctx, "git", "fetch", "--deepen=1")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// configureGitToken sets up git to use the given token for HTTPS authentication
// to github.com. This matches what actions/checkout does and ensures operations
// like `git fetch` succeed even when persist-credentials is not set.
func configureGitToken(ctx context.Context, token string) error {
	// Encode as "x-access-token:<token>" in base64 for HTTP basic auth.
	encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	cmd := exec.CommandContext(ctx, "git", "config", "--global",
		"http.https://github.com/.extraheader",
		"AUTHORIZATION: basic "+encoded)
	return cmd.Run()
}

// changedFile represents a file changed in the diff with its path and added line numbers.
type changedFile struct {
	Path             string
	AddedLines       []string // content of added lines (without the '+' prefix)
	AddedLineNumbers []int    // 1-based line numbers in the new file
}

// resolveBaseSHA returns the base SHA for diffing.
// If baseSHA is provided, use it. Otherwise try HEAD~1; if that fails, attempt to
// deepen a shallow clone and retry, and only fall back to the empty tree when this
// is truly the initial commit. If the base cannot be resolved, an error is returned.
func resolveBaseSHA(ctx context.Context, baseSHA string) (string, error) {
	if baseSHA != "" {
		return baseSHA, nil
	}

	// Try HEAD~1
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD~1")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// If resolving HEAD~1 failed, we may be in a shallow clone. Try to deepen.
	if derr := detectShallowAndDeepen(ctx); derr == nil {
		cmd = exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD~1")
		out, err = cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	// As a final check, see if this is truly the initial commit (a repository with
	// exactly one commit). In that case we diff against the empty tree.
	countCmd := exec.CommandContext(ctx, "git", "rev-list", "--count", "HEAD")
	countOut, countErr := countCmd.Output()
	if countErr == nil && strings.TrimSpace(string(countOut)) == "1" {
		return emptyTreeHash, nil
	}

	return "", fmt.Errorf("failed to resolve base SHA: %w", err)
}

// detectChangedFiles runs git diff and returns the list of changed files with their added lines.
func detectChangedFiles(ctx context.Context, baseSHA string) ([]changedFile, error) {
	base, err := resolveBaseSHA(ctx, baseSHA)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "diff", base, "HEAD", "-U0")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	return parseDiffOutput(string(out))
}

// unquotePath handles git's quoting of filenames with non-ASCII or special characters.
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if unquoted, err := strconv.Unquote(p); err == nil {
			return unquoted
		}
	}
	return p
}

// parseHunkHeader parses "@@ -old,count +new,count @@" and returns the start line and count
// for the new file side.
func parseHunkHeader(line string) (start, count int, ok bool) {
	// Format: @@ -l,s +l,s @@ optional section heading
	plusIdx := strings.Index(line, "+")
	if plusIdx < 0 {
		return 0, 0, false
	}
	rest := line[plusIdx+1:]
	endIdx := strings.Index(rest, " @@")
	if endIdx < 0 {
		// Try just "@@"
		endIdx = strings.Index(rest, "@@")
	}
	if endIdx >= 0 {
		rest = rest[:endIdx]
	}

	parts := strings.SplitN(rest, ",", 2)
	s, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	c := 1
	if len(parts) == 2 {
		c, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	return s, c, true
}

// parseDiffOutput parses unified diff output and extracts changed files with added lines.
func parseDiffOutput(diffOutput string) ([]changedFile, error) {
	var files []changedFile
	var current *changedFile

	scanner := bufio.NewScanner(strings.NewReader(diffOutput))
	// Allow up to 10 MiB per line to handle large diffs without silent truncation.
	const maxScanTokenSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), maxScanTokenSize)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				files = append(files, *current)
			}
			current = &changedFile{}
			continue
		}

		if current != nil && strings.HasPrefix(line, "+++ ") {
			// Unquote first (handles git's C-style quoting), then strip "b/" prefix
			raw := unquotePath(line[4:])
			current.Path = strings.TrimPrefix(raw, "b/")
			continue
		}

		if current != nil && strings.HasPrefix(line, "@@ ") {
			start, count, ok := parseHunkHeader(line)
			if ok {
				for i := 0; i < count; i++ {
					current.AddedLineNumbers = append(current.AddedLineNumbers, start+i)
				}
			}
			continue
		}

		if current != nil && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			current.AddedLines = append(current.AddedLines, line[1:])
		}
	}
	if current != nil {
		files = append(files, *current)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning diff output: %w", err)
	}
	return files, nil
}

// detectPrompts runs the full detection pipeline: shallow deepen, diff, and parse prompts.
// It returns only prompts from newly added @copilot lines.
func detectPrompts(ctx context.Context, baseSHA string) ([]Prompt, error) {
	if err := detectShallowAndDeepen(ctx); err != nil {
		return nil, fmt.Errorf("failed to deepen shallow clone: %w", err)
	}

	changed, err := detectChangedFiles(ctx, baseSHA)
	if err != nil {
		return nil, err
	}

	var prompts []Prompt
	for _, cf := range changed {
		if cf.Path == "/dev/null" {
			continue
		}

		// Build set of added line numbers for fast lookup
		addedSet := make(map[int]bool, len(cf.AddedLineNumbers))
		for _, ln := range cf.AddedLineNumbers {
			addedSet[ln] = true
		}

		// Check if any added line contains @copilot
		hasNewCopilot := false
		for _, line := range cf.AddedLines {
			if strings.HasPrefix(line, "@copilot ") || line == "@copilot" {
				hasNewCopilot = true
				break
			}
		}
		if !hasNewCopilot {
			continue
		}

		// Read the actual file content and parse all prompts
		content, err := os.ReadFile(cf.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", cf.Path, err)
		}
		filePrompts := ParsePrompts(cf.Path, string(content))

		// Filter to only prompts whose StartLine is in the added line numbers
		for _, p := range filePrompts {
			if addedSet[p.StartLine] {
				prompts = append(prompts, p)
			}
		}
	}
	return prompts, nil
}
