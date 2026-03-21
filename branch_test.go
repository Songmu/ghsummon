package ghsummon

import (
	"crypto/md5"
	"fmt"
	"testing"
)

func md5hex(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "notes/memo.md",
			expected: "ghsummon-notes/memo.md",
		},
		{
			name:     "dot-slash prefix",
			input:    "./notes/memo.md",
			expected: "ghsummon-notes/memo.md",
		},
		{
			name:     "root-level file",
			input:    "README.md",
			expected: "ghsummon-README.md",
		},
		{
			name:     "windows-style path separator",
			input:    `notes\memo.md`,
			expected: "ghsummon-" + md5hex(`notes\memo.md`), // backslash is unsafe for git branches
		},
		{
			name:     "japanese characters",
			input:    "日本語ファイル.md",
			expected: "ghsummon-" + md5hex("日本語ファイル.md"),
		},
		{
			name:     "path with spaces",
			input:    "path with spaces/file.md",
			expected: "ghsummon-" + md5hex("path with spaces/file.md"),
		},
		{
			name:     "tilde",
			input:    "path~/file.md",
			expected: "ghsummon-" + md5hex("path~/file.md"),
		},
		{
			name:     "caret",
			input:    "path^/file.md",
			expected: "ghsummon-" + md5hex("path^/file.md"),
		},
		{
			name:     "colon",
			input:    "path:/file.md",
			expected: "ghsummon-" + md5hex("path:/file.md"),
		},
		{
			name:     "question mark",
			input:    "path?/file.md",
			expected: "ghsummon-" + md5hex("path?/file.md"),
		},
		{
			name:     "asterisk",
			input:    "path*/file.md",
			expected: "ghsummon-" + md5hex("path*/file.md"),
		},
		{
			name:     "bracket",
			input:    "path[0]/file.md",
			expected: "ghsummon-" + md5hex("path[0]/file.md"),
		},
		{
			name:     "double dot",
			input:    "path/../file.md",
			expected: "ghsummon-file.md", // filepath.Clean resolves ..
		},
		{
			name:     "ends with .lock",
			input:    "path/file.lock",
			expected: "ghsummon-" + md5hex("path/file.lock"),
		},
		{
			name:     "deeply nested path",
			input:    "a/b/c/d.md",
			expected: "ghsummon-a/b/c/d.md",
		},
		{
			name:     "at-brace",
			input:    "path@{0}/file.md",
			expected: "ghsummon-" + md5hex("path@{0}/file.md"),
		},
		{
			name:     "ends with dot",
			input:    "path/file.",
			expected: "ghsummon-" + md5hex("path/file."),
		},
		{
			name:     "ends with slash",
			input:    "path/dir/",
			expected: "ghsummon-path/dir", // filepath.Clean removes trailing slash
		},
		{
			name:     "dot-slash with windows separator",
			input:    `./notes\memo.md`,
			expected: "ghsummon-" + md5hex(`notes\memo.md`), // backslash is unsafe for git branches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.input)
			if got != tt.expected {
				t.Errorf("BranchName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
