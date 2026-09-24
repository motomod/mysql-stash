package stashers

import (
	"github.com/motomod/mysql-stash/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMySQLDump installs a fake mysqldump on PATH that logs its args and password to a file,
// rejecting the named options like MariaDB's mysqldump does.
func fakeMySQLDump(t *testing.T, rejectedOptions ...string) string {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")
	rejections := ""

	for _, option := range rejectedOptions {
		rejections += `case "$*" in *--` + option + `=*) echo "partial output"; echo "mysqldump: unknown variable '` + option + `=...'" >&2; exit 7 ;; esac
`
	}

	script := `#!/bin/sh
echo "pwd=$MYSQL_PWD args=$*" >> "` + logFile + `"
` + rejections + `case "$*" in
  *baddb*) echo "Got error: 1049: Unknown database 'baddb'" >&2; exit 2 ;;
esac
for db; do :; done
echo "-- dump of $db"
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
	logFile := fakeMySQLDump(t)
	db := &config.DB{Host: "127.0.0.1", Port: 3306, Database: "foo", User: "root", Pass: "p@ss word;$x"}

	out, err := dumpToString(t, db)

	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo\n", out)
	assert.Equal(t, []string{"pwd=p@ss word;$x args=-h 127.0.0.1 -P 3306 -u root --column-statistics=0 --set-gtid-purged=OFF foo"}, readLog(t, logFile))
}

func TestDumpRetriesWithoutColumnStatistics(t *testing.T) {
	logFile := fakeMySQLDump(t, "column-statistics")
	db := &config.DB{Host: "h", Port: 1, Database: "foo", User: "u", Pass: "p"}

	out, err := dumpToString(t, db)

	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo\n", out, "output from the failed first attempt should be discarded")
	assert.Len(t, readLog(t, logFile), 2)
}

func TestDumpRetriesWithoutEachUnsupportedOption(t *testing.T) {
	logFile := fakeMySQLDump(t, "column-statistics", "set-gtid-purged")

	out, err := dumpToString(t, &config.DB{Host: "h", Port: 1, User: "u", Database: "foo"})

	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo\n", out)
	calls := readLog(t, logFile)
	require.Len(t, calls, 3)
	assert.Equal(t, "pwd= args=-h h -P 1 -u u foo", calls[2])
}

func TestDumpKeepsSupportedOptionsWhenOneIsRejected(t *testing.T) {
	logFile := fakeMySQLDump(t, "set-gtid-purged")

	_, err := dumpToString(t, &config.DB{Host: "h", Port: 1, User: "u", Database: "foo"})

	require.NoError(t, err)
	calls := readLog(t, logFile)
	require.Len(t, calls, 2)
	assert.Equal(t, "pwd= args=-h h -P 1 -u u --column-statistics=0 foo", calls[1])
}

func TestDumpRunsOnceWhenColumnStatisticsSupported(t *testing.T) {
	logFile := fakeMySQLDump(t)

	_, err := dumpToString(t, &config.DB{Database: "foo"})

	require.NoError(t, err)
	assert.Len(t, readLog(t, logFile), 1)
}

func TestDumpErrorIncludesClientStderr(t *testing.T) {
	fakeMySQLDump(t)

	_, err := dumpToString(t, &config.DB{Database: "baddb"})

	assert.ErrorContains(t, err, "Unknown database 'baddb'")
}

func TestCreateStashWritesPrivateFile(t *testing.T) {
	fakeMySQLDump(t)
	cfg := &config.Config{BaseDir: t.TempDir()}

	err := NewMySQLStasher(cfg).CreateStash(&config.DB{Database: "foo_db"}, "foo", "snap")

	require.NoError(t, err)
	path, _ := cfg.GetStashFilePath("foo", "snap")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "-- dump of foo_db\n", string(content))

	info, _ := os.Stat(path)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, _ = os.Stat(filepath.Dir(path))
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestFailedCreateStashKeepsExistingStash(t *testing.T) {
	fakeMySQLDump(t)
	cfg := &config.Config{BaseDir: t.TempDir()}
	path, _ := cfg.GetStashFilePath("foo", "snap")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("previous good stash"), 0o600))

	err := NewMySQLStasher(cfg).CreateStash(&config.DB{Database: "baddb"}, "foo", "snap")

	assert.ErrorContains(t, err, "Unknown database 'baddb'")
	content, _ := os.ReadFile(path)
	assert.Equal(t, "previous good stash", string(content))

	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1, "temp file should be cleaned up")
}

// fakeMySQL installs a fake mysql client on PATH that logs each call and its stdin.
func fakeMySQL(t *testing.T) string {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")

	script := `#!/bin/sh
case "$*" in
  *@@character_set_database*) echo "call: $*" >> "` + logFile + `"; printf 'utf8mb4\tutf8mb4_0900_ai_ci\n' ;;
  *) echo "call: $* stdin: $(cat)" >> "` + logFile + `" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mysql"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return logFile
}

func TestApplyStashResetsDatabaseBeforeLoading(t *testing.T) {
	logFile := fakeMySQL(t)
	cfg := &config.Config{BaseDir: t.TempDir()}
	path, _ := cfg.GetStashFilePath("foo", "snap")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("-- dump"), 0o600))
	db := &config.DB{Host: "h", Port: 1, User: "u", Database: "my`db"}

	require.NoError(t, NewMySQLStasher(cfg).ApplyStash(db, "foo", "snap"))

	assert.Equal(t, []string{
		"call: -h h -P 1 -u u -N -B -e SELECT @@character_set_database, @@collation_database my`db",
		"call: -h h -P 1 -u u -e DROP DATABASE `my``db`; CREATE DATABASE `my``db` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci; my`db stdin: ",
		"call: -h h -P 1 -u u my`db stdin: -- dump",
	}, readLog(t, logFile))
}

func TestApplyMissingStashDoesNotTouchDatabase(t *testing.T) {
	logFile := fakeMySQL(t)
	cfg := &config.Config{BaseDir: t.TempDir()}

	err := NewMySQLStasher(cfg).ApplyStash(&config.DB{Database: "foo"}, "foo", "nope")

	assert.ErrorContains(t, err, "doesn't exist")
	assert.NoFileExists(t, logFile)
}
