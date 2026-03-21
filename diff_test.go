package ghsummon

import (
	"reflect"
	"testing"
)

func TestParseDiffOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []changedFile
	}{
		{
			name: "single file with added lines",
			input: `diff --git a/notes/memo.md b/notes/memo.md
index 1234567..abcdefg 100644
--- a/notes/memo.md
+++ b/notes/memo.md
@@ -0,0 +1,2 @@
+@copilot 調べて
+  詳しく調べてほしい
`,
			expected: []changedFile{
				{
					Path: "notes/memo.md",
					AddedLines: []string{
						"@copilot 調べて",
						"  詳しく調べてほしい",
					},
					AddedLineNumbers: []int{1, 2},
				},
			},
		},
		{
			name: "multiple files",
			input: `diff --git a/file1.md b/file1.md
index 1234567..abcdefg 100644
--- a/file1.md
+++ b/file1.md
@@ -0,0 +1 @@
+@copilot prompt1
diff --git a/file2.md b/file2.md
index 1234567..abcdefg 100644
--- a/file2.md
+++ b/file2.md
@@ -0,0 +1 @@
+some regular content
`,
			expected: []changedFile{
				{
					Path:             "file1.md",
					AddedLines:       []string{"@copilot prompt1"},
					AddedLineNumbers: []int{1},
				},
				{
					Path:             "file2.md",
					AddedLines:       []string{"some regular content"},
					AddedLineNumbers: []int{1},
				},
			},
		},
		{
			name:     "empty diff",
			input:    "",
			expected: nil,
		},
		{
			name: "multiple hunks in one file",
			input: `diff --git a/file.md b/file.md
index 1234567..abcdefg 100644
--- a/file.md
+++ b/file.md
@@ -3,0 +4,2 @@
+@copilot first prompt
+  continuation
@@ -10,0 +13 @@
+@copilot second prompt
`,
			expected: []changedFile{
				{
					Path: "file.md",
					AddedLines: []string{
						"@copilot first prompt",
						"  continuation",
						"@copilot second prompt",
					},
					AddedLineNumbers: []int{4, 5, 13},
				},
			},
		},
		{
			name: "path with b/ in name uses +++ line",
			input: `diff --git a/docs/a b/notes.md b/docs/a b/notes.md
index 1234567..abcdefg 100644
--- a/docs/a b/notes.md
+++ b/docs/a b/notes.md
@@ -0,0 +1 @@
+@copilot test
`,
			expected: []changedFile{
				{
					Path:             "docs/a b/notes.md",
					AddedLines:       []string{"@copilot test"},
					AddedLineNumbers: []int{1},
				},
			},
		},
		{
			name: "quoted filename with spaces",
			input: `diff --git a/path with spaces/file.md b/path with spaces/file.md
index 1234567..abcdefg 100644
--- "a/path with spaces/file.md"
+++ "b/path with spaces/file.md"
@@ -0,0 +1 @@
+content
`,
			expected: []changedFile{
				{
					Path:             "path with spaces/file.md",
					AddedLines:       []string{"content"},
					AddedLineNumbers: []int{1},
				},
			},
		},
		{
			name: "deleted file (dev/null)",
			input: `diff --git a/removed.md b/removed.md
deleted file mode 100644
index 1234567..0000000
--- a/removed.md
+++ /dev/null
@@ -1,3 +0,0 @@
-old content
`,
			expected: []changedFile{
				{
					Path: "/dev/null",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDiffOutput(tt.input)
			if err != nil {
				t.Fatalf("parseDiffOutput() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseDiffOutput() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		line      string
		wantStart int
		wantCount int
		wantOK    bool
	}{
		{"@@ -0,0 +1,3 @@", 1, 3, true},
		{"@@ -1 +1 @@", 1, 1, true},
		{"@@ -5,2 +7,4 @@ func foo()", 7, 4, true},
		{"@@ -10,0 +13 @@", 13, 1, true},
		{"@@ -1,3 +0,0 @@", 0, 0, true},
		{"not a hunk", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			start, count, ok := parseHunkHeader(tt.line)
			if ok != tt.wantOK || start != tt.wantStart || count != tt.wantCount {
				t.Errorf("parseHunkHeader(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tt.line, start, count, ok, tt.wantStart, tt.wantCount, tt.wantOK)
			}
		})
	}
}

func TestUnquotePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple/path.md", "simple/path.md"},
		{`"path with spaces/file.md"`, "path with spaces/file.md"},
		{`"src/\343\201\202.go"`, "src/あ.go"},
		{"/dev/null", "/dev/null"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := unquotePath(tt.input)
			if got != tt.expected {
				t.Errorf("unquotePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
