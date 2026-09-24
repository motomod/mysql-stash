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

// A trimmed mysqldump output containing table `t`, table `we“ird` and view `v`.
const sampleDump = "-- MySQL dump\n" +
	"DROP TABLE IF EXISTS `t`;\n" +
	"CREATE TABLE `t` (`id` int);\n" +
	"DROP TABLE IF EXISTS `we``ird`;\n" +
	"CREATE TABLE `we``ird` (`id` int);\n" +
	"/*!50001 DROP VIEW IF EXISTS `v`*/;\n" +
	"/*!50001 CREATE VIEW `v` AS SELECT 1 */;\n"

// fakeMySQL installs a fake mysql client on PATH that logs each call and its stdin. It reports
// the tables in `current` when asked, and fails to load any stdin containing "FAIL".
func fakeMySQL(t *testing.T, current string) string {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "calls.log")
	tablesFile := filepath.Join(dir, "tables")
	require.NoError(t, os.WriteFile(tablesFile, []byte(current), 0o600))

	script := `#!/bin/sh
case "$*" in
  *information_schema.TABLES*) echo "query" >> "` + logFile + `"; cat "` + tablesFile + `"; exit 0 ;;
  *" -e "*) echo "exec: $*" >> "` + logFile + `"; exit 0 ;;
esac
input=$(cat)
echo "load: $*" >> "` + logFile + `"
case "$input" in *FAIL*) echo "ERROR 3546 (HY000) at line 24: @@GLOBAL.GTID_PURGED cannot be changed" >&2; exit 1 ;; esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mysql"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return logFile
}

func writeStash(t *testing.T, content string) *config.Config {
	cfg := &config.Config{BaseDir: t.TempDir()}
	path, _ := cfg.GetStashFilePath("foo", "snap")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return cfg
}

func readCalls(t *testing.T, logFile string) []string {
	b, err := os.ReadFile(logFile)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestApplyDropsOnlyObjectsNotInStashAfterLoading(t *testing.T) {
	logFile := fakeMySQL(t, "t\tBASE TABLE\nwe`ird\tBASE TABLE\nadded later\tBASE TABLE\nv\tVIEW\nnew_view\tVIEW\n")
	cfg := writeStash(t, sampleDump)

	require.NoError(t, NewMySQLStasher(cfg).ApplyStash(&config.DB{Host: "h", Port: 1, User: "u", Database: "foo"}, "foo", "snap"))

	assert.Equal(t, []string{
		"load: -h h -P 1 -u u foo",
		"query",
		"exec: -h h -P 1 -u u -e SET FOREIGN_KEY_CHECKS = 0; DROP VIEW IF EXISTS `new_view`; DROP TABLE IF EXISTS `added later`; foo",
	}, readCalls(t, logFile))
}

func TestApplyWithNothingExtraDropsNothing(t *testing.T) {
	logFile := fakeMySQL(t, "t\tBASE TABLE\nwe`ird\tBASE TABLE\nv\tVIEW\n")
	cfg := writeStash(t, sampleDump)

	require.NoError(t, NewMySQLStasher(cfg).ApplyStash(&config.DB{Database: "foo"}, "foo", "snap"))

	assert.Len(t, readCalls(t, logFile), 2, "load and query only")
}

func TestFailedApplyDropsNothing(t *testing.T) {
	logFile := fakeMySQL(t, "t\tBASE TABLE\nadded later\tBASE TABLE\n")
	cfg := writeStash(t, sampleDump+"FAIL\n")

	err := NewMySQLStasher(cfg).ApplyStash(&config.DB{Database: "foo"}, "foo", "snap")

	assert.ErrorContains(t, err, "GTID_PURGED cannot be changed")
	calls := readCalls(t, logFile)
	assert.Len(t, calls, 1, "only the load should run")
	assert.NotContains(t, strings.Join(calls, "\n"), "DROP")
}

func TestApplyMissingStashDoesNotTouchDatabase(t *testing.T) {
	logFile := fakeMySQL(t, "")
	cfg := &config.Config{BaseDir: t.TempDir()}

	err := NewMySQLStasher(cfg).ApplyStash(&config.DB{Database: "foo"}, "foo", "nope")

	assert.ErrorContains(t, err, "doesn't exist")
	assert.NoFileExists(t, logFile)
}
