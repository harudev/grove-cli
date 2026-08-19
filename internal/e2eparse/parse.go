package e2eparse

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TestCase represents a single test case extracted from a file.
type TestCase struct {
	Type string // "describe", "it", "test"
	Name string
	Line int
}

// TestFile represents a file containing test cases.
type TestFile struct {
	Path  string
	Cases []TestCase
}

var testBlockRe = regexp.MustCompile(`^\s*(describe|it|test)\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`)

// ParseDir scans the directory for e2e test files and extracts test cases.
func ParseDir(baseDir string) ([]TestFile, error) {
	patterns := []string{
		"**/*.e2e.ts",
		"**/*.e2e.tsx",
		"**/*.spec.ts",
		"**/*.spec.tsx",
		"e2e/**/*.ts",
		"e2e/**/*.tsx",
	}

	seen := make(map[string]bool)
	var files []TestFile

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(baseDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			abs, _ := filepath.Abs(match)
			if seen[abs] {
				continue
			}
			seen[abs] = true

			cases, err := parseFile(abs)
			if err != nil || len(cases) == 0 {
				continue
			}

			rel, _ := filepath.Rel(baseDir, abs)
			files = append(files, TestFile{Path: rel, Cases: cases})
		}
	}

	return files, nil
}

func parseFile(path string) ([]TestCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []TestCase
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := testBlockRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		cases = append(cases, TestCase{
			Type: matches[1],
			Name: matches[2],
			Line: lineNum,
		})
	}

	return cases, scanner.Err()
}

// FormatAsComment formats parsed test files as a Jira comment.
func FormatAsComment(files []TestFile) string {
	if len(files) == 0 {
		return "E2E 테스트 케이스를 찾을 수 없습니다."
	}

	var sb strings.Builder
	sb.WriteString("🧪 E2E 테스트 케이스\n\n")

	totalCases := 0
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("📄 %s\n", f.Path))
		for _, tc := range f.Cases {
			indent := "  "
			if tc.Type == "it" || tc.Type == "test" {
				indent = "    "
			}
			sb.WriteString(fmt.Sprintf("%s- [%s] %s (L%d)\n", indent, tc.Type, tc.Name, tc.Line))
			totalCases++
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("총 %d개 파일, %d개 테스트 케이스", len(files), totalCases))
	return sb.String()
}
