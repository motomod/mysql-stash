package stashers

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/motomod/mysql-stash/config"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

	if err = resetDatabase(db); err != nil {
		return fmt.Errorf("resetting db '%s': %w", dbName, err)
	}

	if err = run("mysql", connectionArgs(db), db.Pass, stashFile, io.Discard); err != nil {
		return fmt.Errorf("applying stash to db '%s': %w", dbName, err)
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

// resetDatabase drops and recreates db with its current charset and collation, so applying
// a stash also removes tables created since it was taken.
func resetDatabase(db *config.DB) error {
	var out bytes.Buffer

	query := "SELECT @@character_set_database, @@collation_database"

	if err := run("mysql", connectionArgs(db, "-N", "-B", "-e", query), db.Pass, nil, &out); err != nil {
		return err
	}

	fields := strings.Fields(out.String())

	if len(fields) != 2 {
		return fmt.Errorf("unexpected charset query output %q", out.String())
	}

	name := quoteIdentifier(db.Database)
	reset := fmt.Sprintf("DROP DATABASE %s; CREATE DATABASE %s CHARACTER SET %s COLLATE %s;", name, name, fields[0], fields[1])

	return run("mysql", connectionArgs(db, "-e", reset), db.Pass, nil, io.Discard)
}

func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// connectionArgs returns the client arguments for db, with any extra options before the database name.
// Unset options are omitted, so ~/.my.cnf or the client's defaults apply.
func connectionArgs(db *config.DB, extra ...string) []string {
	var args []string

	if db.Host != "" {
		args = append(args, "-h", db.Host)
	}

	if db.Port != 0 {
		args = append(args, "-P", strconv.Itoa(db.Port))
	}

	if db.User != "" {
		args = append(args, "-u", db.User)
	}

	args = append(args, extra...)

	return append(args, db.Database)
}

// run executes a mysql client binary, passing the password via the environment so it
// never appears in the process list, and includes the client's stderr in any error.
func run(name string, args []string, pass string, stdin io.Reader, stdout io.Writer) error {
	var stderr bytes.Buffer

	cmd := exec.Command(name, args...)
	cmd.Env = passwordEnv(pass, os.Environ())
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

// passwordEnv returns env with the password set for the mysql clients, if there is one.
func passwordEnv(pass string, env []string) []string {
	if pass == "" {
		return env
	}

	return append(env, "MYSQL_PWD="+pass)
}
