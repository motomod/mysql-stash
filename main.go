package main

import (
	"errors"
	"fmt"
	"github.com/motomod/mysql-stash/config"
	"github.com/motomod/mysql-stash/stashers"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const stashAction = "stash"
const applyAction = "apply"
const listAction = "list"
const deleteAction = "delete"
const viewAction = "view"

const usage = `usage:
  mysql-stash stash  <db|all> <stash>   save the current state of a database
  mysql-stash apply  <db|all> <stash>   restore a database from a stash
  mysql-stash list                      list stashes
  mysql-stash view   <db> <stash>       print a stash's SQL
  mysql-stash delete <db> <stash>       delete a stash`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if 0 == len(args) {
		return errors.New(usage)
	}

	action := args[0]
	cfg, err := config.New()

	if err != nil {
		return err
	}

	switch action {
	case listAction:
		return printStashes(cfg)
	case stashAction, applyAction, deleteAction, viewAction:
	default:
		return fmt.Errorf("unrecognised command '%s'\n\n%s", action, usage)
	}

	if len(args) != 3 {
		return fmt.Errorf("'%s' needs a database name and a stash name\n\n%s", action, usage)
	}

	dbName, stashName := args[1], args[2]

	switch action {
	case deleteAction:
		if err := deleteStash(cfg, dbName, stashName); err != nil {
			return err
		}

		fmt.Println("stash deleted")

		return nil
	case viewAction:
		return viewStash(cfg, dbName, stashName)
	}

	databases, err := cfg.LoadDBConfig()

	if err != nil {
		return err
	}

	dbs, err := getDBsFromArgument(dbName, databases)

	if err != nil {
		return err
	}

	stasherInterfaces := map[string]stashers.StasherInterface{
		"mysql": stashers.NewMySQLStasher(cfg),
	}

	stasher := stashers.NewStasher(dbs, stasherInterfaces)

	if stashAction == action {
		if err = stasher.CreateStash(stashName); err != nil {
			return err
		}

		fmt.Printf("Created stash '%s' for %s\n", stashName, describeDBs(dbs))

		return nil
	}

	if err = stasher.ApplyStash(stashName); err != nil {
		return err
	}

	fmt.Printf("Applied stash '%s' to %s\n", stashName, describeDBs(dbs))

	return nil
}

func describeDBs(dbs map[string]*config.DB) string {
	names := make([]string, 0, len(dbs))

	for name := range dbs {
		names = append(names, name)
	}

	sort.Strings(names)

	if len(names) == 1 {
		return "database " + names[0]
	}

	return "databases " + strings.Join(names, ", ")
}

func printStashes(cfg *config.Config) error {
	stashPath := cfg.GetStashPath("")
	dbDirs, err := os.ReadDir(stashPath)

	if errors.Is(err, fs.ErrNotExist) {
		fmt.Println("no stashes")

		return nil
	}

	if err != nil {
		return err
	}

	for _, dbDir := range dbDirs {
		if !dbDir.IsDir() {
			continue
		}

		stashes, err := os.ReadDir(filepath.Join(stashPath, dbDir.Name()))

		if err != nil {
			return err
		}

		var names []string

		for _, stash := range stashes {
			// Skip in-progress dumps, which are written to hidden temp files.
			if !stash.IsDir() && !strings.HasPrefix(stash.Name(), ".") {
				names = append(names, stash.Name())
			}
		}

		if len(names) == 0 {
			continue
		}

		fmt.Println(dbDir.Name())

		for _, name := range names {
			fmt.Printf("- %s\n", name)
		}
	}

	return nil
}

func getDBsFromArgument(dbName string, databases map[string]*config.DB) (map[string]*config.DB, error) {
	if dbName == "all" {
		return databases, nil
	}

	if _, ok := databases[dbName]; ok == false {
		return nil, fmt.Errorf("db '%s' doesn't exist in config", dbName)
	}

	filteredDatabases := make(map[string]*config.DB)
	filteredDatabases[dbName] = databases[dbName]

	return filteredDatabases, nil
}

func deleteStash(cfg *config.Config, dbName string, stashName string) error {
	stashFilePath, err := cfg.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	if _, err := os.Stat(stashFilePath); err != nil {
		return errors.New("stash doesn't exist")
	}

	return os.Remove(stashFilePath)
}

func viewStash(cfg *config.Config, dbName string, stashName string) error {
	stashFilePath, err := cfg.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	stashFile, err := os.Open(stashFilePath)

	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("stash doesn't exist")
	}

	if nil != err {
		return err
	}

	defer stashFile.Close()

	_, err = io.Copy(os.Stdout, stashFile)

	return err
}
