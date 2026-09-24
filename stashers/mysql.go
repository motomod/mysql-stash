package stashers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/motomod/mysql-stash/config"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// mysqldump exits with this code when given an option it doesn't recognise,
// e.g. MariaDB's mysqldump and --column-statistics.
const exitUnknownOption = 7

type MySql struct {
	config *config.Config
}

func NewMySQLStasher(config *config.Config) StasherInterface {
	return &MySql{
		config: config,
	}
}

func (m MySql) CreateStash(db *config.DB, dbName string, stashName string) error {
	stashFilePath, err := m.config.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(stashFilePath), 0o700); err != nil {
		return err
	}

	// Dump to a temp file and only replace the existing stash once the dump has succeeded.
	tmp, err := os.CreateTemp(filepath.Dir(stashFilePath), "."+stashName+".tmp-*")

	if err != nil {
		return err
	}

	defer os.Remove(tmp.Name())

	err = dump(db, tmp)

	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("stashing db '%s': %w", dbName, err)
	}

	return os.Rename(tmp.Name(), stashFilePath)
}

func (m MySql) ApplyStash(db *config.DB, dbName string, stashName string) error {
	stashFilePath, err := m.config.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	stashFile, err := os.Open(stashFilePath)

	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stash '%s' doesn't exist for db '%s'", stashName, dbName)
	}

	if err != nil {
		return err
	}

	defer stashFile.Close()

	stashed, err := stashedObjects(stashFile)

	if err != nil {
		return err
	}

	if _, err = stashFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err = run("mysql", connectionArgs(db), db.Pass, stashFile, io.Discard); err != nil {
		return fmt.Errorf("applying stash to db '%s': %w", dbName, err)
	}

	// Only once the stash has loaded, so a failed apply never removes anything.
	if err = dropObjectsNotIn(db, stashed); err != nil {
		return fmt.Errorf("removing tables created since the stash from db '%s': %w", dbName, err)
	}

	return nil
}

// dump writes a dump of db to f, retrying without --column-statistics=0 if mysqldump doesn't support it.
func dump(db *config.DB, f *os.File) error {
	err := run("mysqldump", connectionArgs(db, "--column-statistics=0"), db.Pass, nil, f)

	var exitErr *exec.ExitError

	if errors.As(err, &exitErr) && exitErr.ExitCode() == exitUnknownOption {
		if err = f.Truncate(0); err != nil {
			return err
		}

		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return err
		}

		err = run("mysqldump", connectionArgs(db), db.Pass, nil, f)
	}

	return err
}

// stashedObjectPattern matches the statements mysqldump writes before each table and view it creates.
var stashedObjectPattern = regexp.MustCompile("^(?:/\\*!50001 )?DROP (?:TABLE|VIEW) IF EXISTS `((?:[^`]|``)+)`")

// stashedObjects returns the names of the tables and views a stash creates.
func stashedObjects(r io.Reader) (map[string]bool, error) {
	objects := map[string]bool{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)

	for scanner.Scan() {
		if match := stashedObjectPattern.FindSubmatch(scanner.Bytes()); match != nil {
			objects[strings.ReplaceAll(string(match[1]), "``", "`")] = true
		}
	}

	return objects, scanner.Err()
}

// dropObjectsNotIn drops the tables and views in db that aren't in stashed, i.e. those created
// since the stash was taken. Stored routines and events aren't in stashes, so are left alone.
func dropObjectsNotIn(db *config.DB, stashed map[string]bool) error {
	var out bytes.Buffer

	query := "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()"

	if err := run("mysql", connectionArgs(db, "-N", "-B", "-e", query), db.Pass, nil, &out); err != nil {
		return err
	}

	var views, tables []string

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		name, tableType, ok := strings.Cut(line, "\t")

		if !ok || stashed[name] {
			continue
		}

		if tableType == "VIEW" {
			views = append(views, "DROP VIEW IF EXISTS "+quoteIdentifier(name)+";")
		} else {
			tables = append(tables, "DROP TABLE IF EXISTS "+quoteIdentifier(name)+";")
		}
	}

	if len(views)+len(tables) == 0 {
		return nil
	}

	// Views first as they may depend on the tables; foreign keys between dropped tables are ignored.
	statements := append(append([]string{"SET FOREIGN_KEY_CHECKS = 0;"}, views...), tables...)

	return run("mysql", connectionArgs(db, "-e", strings.Join(statements, " ")), db.Pass, nil, io.Discard)
}

func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// connectionArgs returns the client arguments for db, with any extra options before the database name.
func connectionArgs(db *config.DB, extra ...string) []string {
	args := []string{"-h", db.Host, "-P", strconv.Itoa(db.Port), "-u", db.User}
	args = append(args, extra...)

	return append(args, db.Database)
}

// run executes a mysql client binary, passing the password via the environment so it
// never appears in the process list, and includes the client's stderr in any error.
func run(name string, args []string, pass string, stdin io.Reader, stdout io.Writer) error {
	var stderr bytes.Buffer

	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// The clients prefix their own messages with their name.
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s (%w)", msg, err)
		}

		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}
