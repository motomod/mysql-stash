package stashers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mysql-stash/config"
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

	if err = run("mysql", connectionArgs(db), db.Pass, stashFile, io.Discard); err != nil {
		return fmt.Errorf("applying stash to db '%s': %w", dbName, err)
	}

	fmt.Printf("Applied stash '%s' for database '%s'\n", stashName, dbName)

	return nil
}

// dump writes a dump of db to f, retrying without --column-statistics=0 if mysqldump doesn't support it.
func dump(db *config.DB, f *os.File) error {
	err := run("mysqldump", append(connectionArgs(db), "--column-statistics=0"), db.Pass, nil, f)

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

func connectionArgs(db *config.DB) []string {
	return []string{"-h", db.Host, "-P", strconv.Itoa(db.Port), "-u", db.User, db.Database}
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
