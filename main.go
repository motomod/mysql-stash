package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mysql-stash/config"
	"mysql-stash/stashers"
	"os"
)

const stashAction = "stash"
const applyAction = "apply"
const listAction = "list"
const deleteAction = "delete"
const viewAction = "view"

func main() {
	argLen := len(os.Args[1:])

	if 0 == argLen {
		fmt.Println("see readme for examples")

		return
	}

	action := os.Args[1]
	config := config.New()

	if listAction == action {
		printStashes(&config)

		return
	}

	if stashAction != action && applyAction != action && deleteAction != action && viewAction != action {
		fmt.Println("unrecognised command, must be 'stash', 'restore', 'delete' or 'view")
		os.Exit(1)
	}

	if 1 == argLen {
		fmt.Println("missing database name")

		return
	}

	dbName := os.Args[2]
	databases, err := config.LoadDBConfig()

	if err != nil {
		fmt.Println("config error")

		return
	}

	if 2 == argLen {
		fmt.Println("missing stash name")

		return
	}

	stashName := os.Args[3]

	if deleteAction == action {
		err = deleteStash(&config, dbName, stashName)

		if err != nil {
			fmt.Println(err)

			return
		}

		fmt.Println("stash deleted")

		return
	}

	if viewAction == action {
		err = viewStash(&config, dbName, stashName)

		if err != nil {
			fmt.Println(err)

			return
		}

		return
	}

	dbs, err := getDBsFromArgument(dbName, databases)

	if err != nil {
		log.Println(err)

		return
	}

	stasherInterfaces := map[string]stashers.StasherInterface{
		"mysql": stashers.NewMySQLStasher(&config),
	}

	stasher := stashers.NewStasher(&config, dbs, stasherInterfaces)

	if stashAction == action {
		err = stasher.CreateStash(dbName, stashName)
	}

	if applyAction == action {
		err = stasher.ApplyStash(dbName, stashName)
	}

	if err != nil {
		fmt.Println(err)
	}
}

func printStashes(config *config.Config) {
	stashPath, err := config.GetStashPath("")

	if err != nil {
		fmt.Println(err)
	}

	dbNames, _ := ioutil.ReadDir(stashPath)

	for _, folder := range dbNames {

		stashes, _ := ioutil.ReadDir(stashPath + "/" + folder.Name())

		if len(stashes) > 0 {
			fmt.Println(folder.Name())
		}

		for _, stash := range stashes {
			fmt.Printf("- %s\n", stash.Name())
		}
	}
}

func getDBsFromArgument(dbName string, databases map[string]*config.DB) (map[string]*config.DB, error) {
	if dbName == "all" {
		return databases, nil
	}

	if _, ok := databases[dbName]; ok == false {
		return nil, errors.New("provided db name doesn't exist in config")
	}

	filteredDatabases := make(map[string]*config.DB)
	filteredDatabases[dbName] = databases[dbName]

	return filteredDatabases, nil
}

func deleteStash(config *config.Config, dbName string, stashName string) error {
	stashFilePath, err := config.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	if _, err := os.Stat(stashFilePath); err != nil {
		return errors.New("stashers doesn't exist")
	}

	return os.Remove(stashFilePath)
}

func viewStash(config *config.Config, dbName string, stashName string) (err error) {
	stashFilePath, err := config.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	bytes, err := ioutil.ReadFile(stashFilePath)

	if nil != err {
		return err
	}

	fmt.Println(string(bytes))

	return err
}
