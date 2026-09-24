package stashers

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mysql-stash/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMySQLDump installs a fake mysqldump on PATH that logs its args and password to a file,
// optionally rejecting --column-statistics like MariaDB does.
func fakeMySQLDump(t *testing.T, rejectColumnStatistics bool) string {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")
	reject := "false"

	if rejectColumnStatistics {
		reject = "true"
	}

	script := `#!/bin/sh
echo "pwd=$MYSQL_PWD args=$*" >> "` + logFile + `"
case "$*" in
  *--column-statistics=0*) if ` + reject + `; then echo "partial output"; echo "unknown variable 'column-statistics=0'" >&2; exit 7; fi ;;
esac
case "$*" in
  *baddb*) echo "Got error: 1049: Unknown database 'baddb'" >&2; exit 2 ;;
esac
echo "-- dump of $7"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mysqldump"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return logFile
}

func dumpToString(t *testing.T, db *config.DB) (string, error) {
	f, err := os.CreateTemp(t.TempDir(), "dump")
	require.NoError(t, err)
	defer f.Close()

	err = dump(db, f)
	out, readErr := os.ReadFile(f.Name())
	require.NoError(t, readErr)

	return string(out), err
}

func readLog(t *testing.T, logFile string) []string {
	b, err := os.ReadFile(logFile)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestDumpPassesPasswordViaEnvironmentOnly(t *testing.T) {
	logFile := fakeMySQLDump(t, false)
	db := &config.DB{Host: "127.0.0.1", Port: 3306, Database: "foo", User: "root", Pass: "p@ss word;$x"}

	out, err := dumpToString(t, db)

	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo\n", out)
	assert.Equal(t, []string{"pwd=p@ss word;$x args=-h 127.0.0.1 -P 3306 -u root foo --column-statistics=0"}, readLog(t, logFile))
}

func TestDumpRetriesWithoutColumnStatistics(t *testing.T) {
	logFile := fakeMySQLDump(t, true)
	db := &config.DB{Host: "h", Port: 1, Database: "foo", User: "u", Pass: "p"}

	out, err := dumpToString(t, db)

	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo\n", out, "output from the failed first attempt should be discarded")
	assert.Len(t, readLog(t, logFile), 2)
}

func TestDumpRunsOnceWhenColumnStatisticsSupported(t *testing.T) {
	logFile := fakeMySQLDump(t, false)

	_, err := dumpToString(t, &config.DB{Database: "foo"})

	require.NoError(t, err)
	assert.Len(t, readLog(t, logFile), 1)
}

func TestDumpErrorIncludesClientStderr(t *testing.T) {
	fakeMySQLDump(t, false)

	_, err := dumpToString(t, &config.DB{Database: "baddb"})

	assert.ErrorContains(t, err, "Unknown database 'baddb'")
}
