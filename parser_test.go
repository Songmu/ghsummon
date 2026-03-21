package ghsummon

import (
	"testing"
)

func TestParsePrompts(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		want    []Prompt
	}{
		{
			name:    "single-line prompt",
			file:    "notes.md",
			content: "@copilot 調べて",
			want: []Prompt{
				{FilePath: "notes.md", StartLine: 1, EndLine: 1, Text: "調べて"},
			},
		},
		{
			name:    "multi-line with tab indent",
			file:    "notes.md",
			content: "@copilot Investigate this\n\tline two\n\tline three",
			want: []Prompt{
				{FilePath: "notes.md", StartLine: 1, EndLine: 3, Text: "Investigate this\nline two\nline three"},
			},
		},
		{
			name:    "multi-line with 2-space indent",
			file:    "notes.md",
			content: "@copilot Investigate this\n  line two\n  line three",
			want: []Prompt{
				{FilePath: "notes.md", StartLine: 1, EndLine: 3, Text: "Investigate this\nline two\nline three"},
			},
		},
		{
			name:    "multi-line with 4-space indent",
			file:    "notes.md",
			content: "@copilot Investigate this\n    line two\n    line three",
			want: []Prompt{
				{FilePath: "notes.md", StartLine: 1, EndLine: 3, Text: "Investigate this\nline two\nline three"},
			},
		},
		{
			name:    "multiple prompts in one file",
			file:    "multi.md",
			content: "@copilot first prompt\n\nsome text\n\n@copilot second prompt\n  with continuation",
			want: []Prompt{
				{FilePath: "multi.md", StartLine: 1, EndLine: 1, Text: "first prompt"},
				{FilePath: "multi.md", StartLine: 5, EndLine: 6, Text: "second prompt\nwith continuation"},
			},
		},
		{
			name:    "no prompt returns empty slice",
			file:    "empty.md",
			content: "just some text\nno copilot here",
			want:    nil,
		},
		{
			name:    "copilot without space after",
			file:    "bare.md",
			content: "@copilot\nnext line",
			want:    nil,
		},
		{
			name:    "copilot in middle of line does not match",
			file:    "inline.md",
			content: "foo @copilot bar",
			want:    nil,
		},
		{
			name:    "mixed content with prompts",
			file:    "mixed.md",
			content: "# Title\n\nSome intro text.\n\n@copilot Research topic A\n  details about A\n\nMore regular text here.\n\n@copilot Research topic B\n\nFinal text.",
			want: []Prompt{
				{FilePath: "mixed.md", StartLine: 5, EndLine: 6, Text: "Research topic A\ndetails about A"},
				{FilePath: "mixed.md", StartLine: 10, EndLine: 10, Text: "Research topic B"},
			},
		},
		{
			name:    "prompt at end of file without trailing newline",
			file:    "eof.md",
			content: "some text\n@copilot last prompt",
			want: []Prompt{
				{FilePath: "eof.md", StartLine: 2, EndLine: 2, Text: "last prompt"},
			},
		},
		{
			name:    "prompt at end of file with continuation no trailing newline",
			file:    "eof2.md",
			content: "some text\n@copilot last prompt\n\tcontinued",
			want: []Prompt{
				{FilePath: "eof2.md", StartLine: 2, EndLine: 3, Text: "last prompt\ncontinued"},
			},
		},
		{
			name:    "continuation ends at non-indented line",
			file:    "stop.md",
			content: "@copilot do something\n  continued\nnot continued",
			want: []Prompt{
				{FilePath: "stop.md", StartLine: 1, EndLine: 2, Text: "do something\ncontinued"},
			},
		},
		{
			name:    "continuation ends at blank line",
			file:    "blank.md",
			content: "@copilot do something\n  continued\n\nnext paragraph",
			want: []Prompt{
				{FilePath: "blank.md", StartLine: 1, EndLine: 2, Text: "do something\ncontinued"},
			},
		},
		{
			name:    "single space indent is not continuation",
			file:    "onespace.md",
			content: "@copilot do something\n not a continuation",
			want: []Prompt{
				{FilePath: "onespace.md", StartLine: 1, EndLine: 1, Text: "do something"},
			},
		},
		{
			name:    "empty content",
			file:    "empty.md",
			content: "",
			want:    nil,
		},
		{
			name:    "sketch example multi-line",
			file:    "sketch.md",
			content: "@copilot GitHub Copilot Coding Agentのトリガー方法について調べて\n  REST API、GraphQL、GitHub CLIそれぞれの方法を比較して\n  ベストプラクティスもまとめてほしい",
			want: []Prompt{
				{
					FilePath:  "sketch.md",
					StartLine: 1,
					EndLine:   3,
					Text:      "GitHub Copilot Coding Agentのトリガー方法について調べて\nREST API、GraphQL、GitHub CLIそれぞれの方法を比較して\nベストプラクティスもまとめてほしい",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePrompts(tt.file, tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d prompts, want %d\ngot:  %+v\nwant: %+v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i].FilePath != tt.want[i].FilePath {
					t.Errorf("prompt[%d].FilePath = %q, want %q", i, got[i].FilePath, tt.want[i].FilePath)
				}
				if got[i].StartLine != tt.want[i].StartLine {
					t.Errorf("prompt[%d].StartLine = %d, want %d", i, got[i].StartLine, tt.want[i].StartLine)
				}
				if got[i].EndLine != tt.want[i].EndLine {
					t.Errorf("prompt[%d].EndLine = %d, want %d", i, got[i].EndLine, tt.want[i].EndLine)
				}
				if got[i].Text != tt.want[i].Text {
					t.Errorf("prompt[%d].Text = %q, want %q", i, got[i].Text, tt.want[i].Text)
				}
			}
		})
	}
}
