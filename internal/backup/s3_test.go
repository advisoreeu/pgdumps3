package backup

import (
	"io"
	"strings"
	"testing"
)

func TestCountingReader(t *testing.T) {
	t.Parallel()

	const data = "hello world, this is a backup stream"

	cr := &countingReader{r: strings.NewReader(data)}

	n, err := io.Copy(io.Discard, cr)
	if err != nil {
		t.Fatalf("io.Copy failed: %v", err)
	}

	if n != int64(len(data)) {
		t.Errorf("io.Copy returned %d bytes, want %d", n, len(data))
	}

	if got := cr.count.Load(); got != int64(len(data)) {
		t.Errorf("counter = %d, want %d", got, len(data))
	}
}
