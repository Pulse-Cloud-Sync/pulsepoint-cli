package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// PulsePointIgnoreMatcher handles gitignore-style pattern matching
type PulsePointIgnoreMatcher struct {
	patterns []Pattern
}

// Pattern represents a single ignore pattern
type Pattern struct {
	Pattern    string
	IsNegation bool // Patterns starting with !
	IsDir      bool // Patterns ending with /
}

// NewPulsePointIgnoreMatcher creates a new ignore matcher
func NewPulsePointIgnoreMatcher() *PulsePointIgnoreMatcher {
	return &PulsePointIgnoreMatcher{
		patterns: []Pattern{},
	}
}

// LoadFromFile loads ignore patterns from a file (like .gitignore)
func (m *PulsePointIgnoreMatcher) LoadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, that's okay
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var patterns []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	m.AddPatterns(patterns)
	return scanner.Err()
}

// AddPatterns adds multiple patterns to the matcher
func (m *PulsePointIgnoreMatcher) AddPatterns(patterns []string) {
	for _, pattern := range patterns {
		m.AddPattern(pattern)
	}
}

// AddPattern adds a single pattern to the matcher
func (m *PulsePointIgnoreMatcher) AddPattern(pattern string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.HasPrefix(pattern, "#") {
		return
	}

	p := Pattern{
		Pattern: pattern,
	}

	// Check for negation
	if strings.HasPrefix(pattern, "!") {
		p.IsNegation = true
		p.Pattern = pattern[1:]
	}

	// Check if directory-only pattern
	if strings.HasSuffix(p.Pattern, "/") {
		p.IsDir = true
		p.Pattern = strings.TrimSuffix(p.Pattern, "/")
	}

	m.patterns = append(m.patterns, p)
}

// ShouldIgnore checks if a path should be ignored based on the patterns
func (m *PulsePointIgnoreMatcher) ShouldIgnore(path string, isDir bool) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Default ignore patterns (always applied)
	base := filepath.Base(path)
	if m.pulsePointIsDefaultIgnored(base) {
		return true
	}

	// Check user patterns
	ignored := false
	for _, pattern := range m.patterns {
		// Skip directory-only patterns if checking a file
		if pattern.IsDir && !isDir {
			continue
		}

		if m.pulsePointMatches(path, pattern.Pattern) {
			if pattern.IsNegation {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

// GetPatterns returns all configured patterns
func (m *PulsePointIgnoreMatcher) GetPatterns() []string {
	result := make([]string, len(m.patterns))
	for i, p := range m.patterns {
		pattern := p.Pattern
		if p.IsNegation {
			pattern = "!" + pattern
		}
		if p.IsDir {
			pattern = pattern + "/"
		}
		result[i] = pattern
	}
	return result
}

// pulsePointMatches checks if a path matches a pattern
func (m *PulsePointIgnoreMatcher) pulsePointMatches(path, pattern string) bool {
	// Convert pattern to use forward slashes
	pattern = filepath.ToSlash(pattern)

	// Simple glob matching
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Also try matching against the full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	// Check if the path contains the pattern as a substring
	if strings.Contains(path, pattern) {
		return true
	}

	// Check if the pattern matches the base name exactly
	if filepath.Base(path) == pattern {
		return true
	}

	// Check if pattern matches any parent directory
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == pattern {
			return true
		}
		if matched, _ := filepath.Match(pattern, part); matched {
			return true
		}
	}

	return false
}

// pulsePointIsDefaultIgnored checks if a file should be ignored by default
func (m *PulsePointIgnoreMatcher) pulsePointIsDefaultIgnored(name string) bool {
	defaultIgnores := []string{
		".DS_Store",
		"Thumbs.db",
		"desktop.ini",
		".git",
		".svn",
		".hg",
		".idea",
		".vscode",
		"node_modules",
		"__pycache__",
		"*.pyc",
		"*.pyo",
		"*.swp",
		"*.swo",
		"*~",
		"#*#",
		".#*",
	}

	for _, pattern := range defaultIgnores {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}

	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp")
}
