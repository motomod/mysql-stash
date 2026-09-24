package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

// HomeEnvVar overrides the default ~/.config/mysql-stash storage directory.
const HomeEnvVar = "MYSQL_STASH_HOME"

const configFilename = "config.yml"
const stashDirname = "stashes"

type Config struct {
	// BaseDir holds config.yml and the stashes directory.
	BaseDir string
}

type DB struct {
	Host     string
	Port     int
	Database string
	User     string
	Pass     string
}

type configFile struct {
	Databases map[string]*DB
}

// New returns a Config rooted at $MYSQL_STASH_HOME, or ~/.config/mysql-stash if unset.
func New() (*Config, error) {
	if baseDir := os.Getenv(HomeEnvVar); baseDir != "" {
		return &Config{BaseDir: baseDir}, nil
	}

	homeDir, err := os.UserHomeDir()

	if err != nil {
		return nil, err
	}

	return &Config{BaseDir: filepath.Join(homeDir, ".config", "mysql-stash")}, nil
}

func (c *Config) ConfigFilePath() string {
	return filepath.Join(c.BaseDir, configFilename)
}

func (c *Config) LoadDBConfig() (map[string]*DB, error) {
	path := c.ConfigFilePath()
	yamlFile, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no config found at %s, see config-example.yml", path)
	}

	if err != nil {
		return nil, err
	}

	cnf := configFile{}

	if err = yaml.Unmarshal(yamlFile, &cnf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return cnf.Databases, nil
}

// GetStashPath returns the directory holding a database's stashes, or all stashes if dbName is empty.
func (c *Config) GetStashPath(dbName string) string {
	return filepath.Join(c.BaseDir, stashDirname, dbName)
}

// GetStashFilePath returns the path of a stash file, rejecting names that would escape the stash directory.
func (c *Config) GetStashFilePath(dbName string, stashName string) (string, error) {
	if err := validateName("database", dbName); err != nil {
		return "", err
	}

	if err := validateName("stash", stashName); err != nil {
		return "", err
	}

	return filepath.Join(c.GetStashPath(dbName), stashName), nil
}

func validateName(kind string, name string) error {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid %s name '%s'", kind, name)
	}

	return nil
}
