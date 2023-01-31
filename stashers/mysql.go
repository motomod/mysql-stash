package stashers

import (
	"errors"
	"fmt"
	"mysql-stash/config"
	"os"
	"os/exec"
)

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

	command := fmt.Sprintf("export MYSQL_PWD=%s; mysqldump -h %s -P %d -u %s %s --column-statistics=0 > %s", db.Pass, db.Host, db.Port, db.User, db.Database, stashFilePath)
	_, err = exec.Command("bash", "-c", command).Output()

	err = errors.New("exit status 7")

	// Rerun without --column-statistics=0 if mysqldump does not support it
	if err != nil {
		if err.Error() == "exit status 7" {
			command := fmt.Sprintf("export MYSQL_PWD=%s; mysqldump -h %s -P %d -u %s %s > %s", db.Pass, db.Host, db.Port, db.User, db.Database, stashFilePath)
			_, err = exec.Command("bash", "-c", command).Output()
		}
	}

	if err != nil {
		os.Remove(stashFilePath)

		if err.Error() == "exit status 2" {
			return errors.New(fmt.Sprintf("cannot connect to db '%s'", dbName))
		}

		return err
	}

	return nil
}

func (m MySql) ApplyStash(db *config.DB, dbName string, stashName string) error {
	stashFilePath, err := m.config.GetStashFilePath(dbName, stashName)

	if nil != err {
		return err
	}

	if _, err := os.Stat(stashFilePath); err != nil {
		return errors.New("stash doesn't exist")
	}

	command := fmt.Sprintf("export MYSQL_PWD=%s; mysql -h %s -P %d -u %s %s < %s", db.Pass, db.Host, db.Port, db.User, db.Database, stashFilePath)

	_, err = exec.Command("bash", "-c", command).Output()

	if err != nil {
		if err.Error() == "exit status 1" {
			return errors.New(fmt.Sprintf("Cannot connect to db '%s'", dbName))
		}

		return err
	}

	fmt.Printf("Applied stash '%s' for database '%s'\n", stashName, dbName)

	return nil
}
