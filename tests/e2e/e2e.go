package e2e

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func ExtractBackupFilename(r io.ReadCloser, host string) (string, error) {
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// print every line as it's read
		fmt.Println(line)

		idx := strings.Index(line, host)
		if idx == -1 {
			continue
		}

		// position after the host
		pos := idx + len(host)

		// skip an optional single '/' that follows the host
		if pos < len(line) && line[pos] == '/' {
			pos++
		}

		if pos >= len(line) {
			return "", fmt.Errorf("found host %q but no path after it", host)
		}

		rest := line[pos:]

		// remove trailing characters that commonly appear in logs: closing quote, brace, comma, whitespace
		rest = strings.TrimRight(rest, `"' },`)

		// remove any leading slashes just in case
		rest = strings.TrimLeft(rest, "/")

		if rest == "" {
			return "", fmt.Errorf("found host %q but couldn't parse path after it", host)
		}

		// get the last path segment (the filename)
		parts := strings.SplitN(rest, "/", 2)
		filename := parts[len(parts)-1]
		// extra safety trim
		filename = strings.Trim(filename, `"' },`)

		if filename == "" {
			return "", fmt.Errorf("couldn't extract filename after host %q", host)
		}

		// return the first filename found
		return filename, nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning logs: %w", err)
	}
	return "", fmt.Errorf("no occurrences of host %q found in logs", host)
}

func ReplaceBuildPlatform(folder, dockerfileName, dockerFileAfter string) error {
	// Construct full path
	filePath := filepath.Join(folder, dockerfileName)

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Build actual platform string
	actualPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	replacement := fmt.Sprintf(`--platform="%s"`, actualPlatform)

	// Replace occurrences
	updated := strings.ReplaceAll(string(content), "--platform=$BUILDPLATFORM", replacement)

	filePath = filepath.Join(folder, dockerFileAfter)
	// Write updated content back to file
	if err := os.WriteFile(filePath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
