package mysql

import (
	"errors"
	"fmt"
	"mysql-stash/config"
	"os"
	"os/exec"
)

type MySqlInterface interface {
	ApplyStash(db *config.DB, dbName string, stashName string) error
	CreateStash(db *config.DB, dbName string, stashName string) error
}

type MySql struct {
	config *config.Config
}

func New(config *config.Config) *MySql {
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

	if err != nil {
		if err.Error() == "exit status 2" {
			return errors.New(fmt.Sprintf("Cannot connect to db '%s'", dbName))
		}

		return err
	}

	fmt.Printf("Created stash '%s' for database '%s'\n", stashName, dbName)

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
