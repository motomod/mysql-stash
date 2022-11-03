package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
)

const localStoragePath = "./"
const homeStoragePathFormat = "%s/.config/mysql-stash/"
const configFilename = "config.yml"
const stashPath = "stashes/"

type Config struct {
	Databases map[string]*DB
}

type DB struct {
	Host     string
	Port     int
	Database string
	User     string
	Pass     string
}

func New() Config {
	return Config{}
}

func (c Config) LoadDBConfig() (map[string]*DB, error) {
	filepath, err := c.getConfigFileLoc()

	if err != nil {
		return nil, err
	}

	yamlFile, err := ioutil.ReadFile(filepath)

	if err != nil {
		return nil, err
	}

	cnf := Config{}

	err = yaml.Unmarshal(yamlFile, &cnf)

	if err != nil {
		return nil, err
	}

	return c.Databases, err
}

func (c Config) getStoragePath() (string, error) {
	homePath, err := os.UserHomeDir()

	if nil != err {
		return "", err
	}

	homeStoragePath := fmt.Sprintf(homeStoragePathFormat, homePath)

	if _, err := os.Stat(homeStoragePath); err == nil {
		return homeStoragePath, nil
	}

	if _, err := os.Stat(localStoragePath); err == nil {
		return localStoragePath, nil
	}

	return "", errors.New("no suitable storage directory exists")
}

func (c Config) GetStashPath(dbName string) (string, error) {
	storagePath, err := c.getStoragePath()

	if err != nil {
		return "", err
	}

	return filepath.Join(storagePath, stashPath, dbName), nil
}

func (c Config) GetStashFilePath(dbName string, stashName string) (string, error) {
	stashPath, err := c.GetStashPath(dbName)
	stashFilePath := fmt.Sprintf("%s/%s", stashPath, stashName)

	if err != nil {
		return "", err
	}

	err = os.MkdirAll(stashPath, os.ModePerm)

	if err != nil {
		return "", err
	}

	return stashFilePath, nil
}

func (c Config) getConfigFileLoc() (string, error) {
	storagePath, err := c.getStoragePath()

	if err != nil {
		return "", err
	}

	if _, err := os.Stat(storagePath + configFilename); err == nil {
		return storagePath + configFilename, nil
	}

	return "", errors.New("no config exists")
}
