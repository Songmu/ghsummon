package ghsummon

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
	"strings"
)

const branchPrefix = "ghsummon-"

// BranchName returns the ghsummon branch name for the given file path.
func BranchName(filePath string) string {
	// Normalize the path using OS-native rules, then convert all path
	// separators to forward slashes. This ensures branch names are consistent
	// regardless of the OS (e.g. backslashes on Windows become slashes).
	normalized := filepath.ToSlash(filepath.Clean(filePath))

	// If unsafe characters are present, replace with MD5 hash
	if hasUnsafeChars(normalized) {
		hash := md5.Sum([]byte(normalized))
		normalized = fmt.Sprintf("%x", hash)
	}

	return branchPrefix + normalized
}

func hasUnsafeChars(s string) bool {
	// Check for ASCII control characters and other unsafe single characters
	for _, r := range s {
		if r < 0x20 || r == 0x7f { // ASCII control characters
			return true
		}
		if r > 0x7e { // non-ASCII characters
			return true
		}
		switch r {
		case ' ', '~', '^', ':', '?', '*', '[', '\\':
			return true
		}
	}
	// Check for ".." (double dot)
	if strings.Contains(s, "..") {
		return true
	}
	// Check for "@{"
	if strings.Contains(s, "@{") {
		return true
	}
	// Check if it ends with ".lock", "/", or "."
	if strings.HasSuffix(s, ".lock") ||
		strings.HasSuffix(s, "/") ||
		strings.HasSuffix(s, ".") {
		return true
	}
	return false
}
