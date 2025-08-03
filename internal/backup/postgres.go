package backup

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

type Postgres struct {
	config       *Config
	PGDumpPath   string
	MajorVersion int
}

func NewPostgres(config *Config) *Postgres {
	return &Postgres{
		config: config,
	}
}

const (
	HighestVersion = 17
	LowestVersion  = 15
)

// SetVersion detects the PostgreSQL server version using psql.
func (p *Postgres) SetVersion() error {
	psqlPath := fmt.Sprintf("/usr/libexec/postgresql%d/psql", HighestVersion)
	slog.Info("Detecting PostgreSQL version", "psql_path", psqlPath)

	cmd := exec.Command(psqlPath,
		"-h", p.config.DBHost,
		"-p", strconv.Itoa(p.config.DBPort),
		"-U", p.config.DBUser,
		"-d", p.config.DBName,
		"-t",
		"-A",
		"-c", "SELECT current_setting('server_version_num')::int",
	)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.config.DBPassword))

	var (
		out    bytes.Buffer
		errOut bytes.Buffer
	)

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to get PostgreSQL version: %w, stderr: %s", err, errOut.String())
	}

	versionStr := strings.TrimSpace(out.String())
	slog.Debug("postgres version string", "stdout", versionStr)

	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("failed to parse version number %s: %w", versionStr, err)
	}
	// https://www.postgresql.org/docs/current/functions-info.html#FUNCTIONS-INFO-VERSION
	const versionMultiplier int = 10000
	if versionNum >= LowestVersion*versionMultiplier {
		p.MajorVersion = versionNum / versionMultiplier
	} else {
		return fmt.Errorf("Postgres version is not supported found %d min %d", versionNum, LowestVersion*versionMultiplier)
	}

	slog.Info("Detected PostgreSQL major version", "version", p.MajorVersion)

	return p.setPGDumpPath()
}

// setPGDumpPath sets the path to the appropriate pg_dump binary.
func (p *Postgres) setPGDumpPath() error {
	for v := p.MajorVersion; v <= HighestVersion; v++ {
		candidatePath := fmt.Sprintf("/usr/libexec/postgresql%d/pg_dump", v)
		if fileExists(candidatePath) {
			p.PGDumpPath = candidatePath
			slog.Info("Found suitable pg_dump", "path", p.PGDumpPath)

			return nil
		}
	}

	return fmt.Errorf("no suitable pg_dump found for PostgreSQL %d", p.MajorVersion)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	cmd := exec.Command("test", "-f", path)
	return cmd.Run() == nil
}
