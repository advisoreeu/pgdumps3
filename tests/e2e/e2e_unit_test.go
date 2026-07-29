package e2e

import (
	"io"
	"strings"
	"testing"
)

func TestExtractBackupFilename(t *testing.T) {
	t.Parallel()

	const want = "backups/pg18_postgres_2026-07-29T18-36-00_dev.sql.gz"

	cases := []struct {
		name string
		line string
	}{
		{
			name: "location followed by trailing fields",
			line: `{"level":"INFO","msg":"Successfully uploaded backup to S3",` +
				`"location":"http://minio:9000/test-bucket/` + want + `","bytes":2811840}`,
		},
		{
			name: "location as last field",
			line: `{"msg":"Successfully uploaded backup to S3",` +
				`"location":"http://minio:9000/test-bucket/` + want + `"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractBackupFilename(io.NopCloser(strings.NewReader(tc.line)), "minio:9000")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
