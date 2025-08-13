package e2e

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func ExtractBackupFilename(r io.ReadCloser, host string) (string, error) {
	defer func() {
		if err := r.Close(); err != nil {
			fmt.Printf("failed to close reader: %v\n", err)
		}
	}()

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

		const splitLimit = 2

		parts := strings.SplitN(rest, "/", splitLimit)
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
