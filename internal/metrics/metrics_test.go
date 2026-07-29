package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordSuccess(t *testing.T) {
	t.Parallel()

	m := New("testdb")
	m.RecordSuccess(1234, 2*time.Second)

	if got := testutil.ToFloat64(m.runsTotal.WithLabelValues("success")); got != 1 {
		t.Errorf("success counter = %v, want 1", got)
	}

	if got := testutil.ToFloat64(m.runsTotal.WithLabelValues("failure")); got != 0 {
		t.Errorf("failure counter = %v, want 0", got)
	}

	if got := testutil.ToFloat64(m.lastBackupSize); got != 1234 {
		t.Errorf("last backup size = %v, want 1234", got)
	}

	if got := testutil.ToFloat64(m.lastSuccessTime); got == 0 {
		t.Error("last success timestamp was not set")
	}
}

func TestRecordFailure(t *testing.T) {
	t.Parallel()

	m := New("testdb")
	m.RecordFailure(time.Second)

	if got := testutil.ToFloat64(m.runsTotal.WithLabelValues("failure")); got != 1 {
		t.Errorf("failure counter = %v, want 1", got)
	}

	if got := testutil.ToFloat64(m.lastBackupSize); got != 0 {
		t.Errorf("last backup size = %v, want 0 after failure", got)
	}
}

func TestSetInProgress(t *testing.T) {
	t.Parallel()

	m := New("testdb")

	m.SetInProgress(true)

	if got := testutil.ToFloat64(m.backupInProgress); got != 1 {
		t.Errorf("in_progress = %v, want 1", got)
	}

	m.SetInProgress(false)

	if got := testutil.ToFloat64(m.backupInProgress); got != 0 {
		t.Errorf("in_progress = %v, want 0", got)
	}
}

func TestSetBuildInfo(t *testing.T) {
	t.Parallel()

	m := New("testdb")
	m.SetBuildInfo("v1.2.3", 17)

	if got := testutil.ToFloat64(m.buildInfo.WithLabelValues("v1.2.3", "17")); got != 1 {
		t.Errorf("build_info = %v, want 1", got)
	}
}

func TestServerServesMetrics(t *testing.T) {
	t.Parallel()

	m := New("testdb")
	m.SetBuildInfo("v0.0.0", 16)
	m.RecordSuccess(4096, time.Second)

	srv := httptest.NewServer(m.NewServer("").Handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics") //nolint:noctx // test-local request
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	out := string(body)

	want := []string{
		`pgdumps3_backup_runs_total{database="testdb",status="success"} 1`,
		`pgdumps3_last_backup_size_bytes{database="testdb"} 4096`,
		"pgdumps3_last_success_timestamp_seconds",
		"pgdumps3_backup_duration_seconds_bucket",
		"pgdumps3_backup_in_progress",
		`pgdumps3_build_info{database="testdb",pg_version="16",version="v0.0.0"} 1`,
		// The four default runtime metrics we deliberately keep.
		"go_goroutines",
		"process_start_time_seconds",
		"process_resident_memory_bytes",
		"process_open_fds",
		"process_max_fds",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("/metrics output missing %q", w)
		}
	}

	// Default metrics we trimmed away must not be exposed.
	notWant := []string{
		"go_memstats_",
		"go_gc_",
		"go_threads",
		"go_info",
		"process_cpu_seconds_total",
		"process_network_",
		"process_virtual_memory_",
	}
	for _, nw := range notWant {
		if strings.Contains(out, nw) {
			t.Errorf("/metrics output should not contain %q", nw)
		}
	}
}
