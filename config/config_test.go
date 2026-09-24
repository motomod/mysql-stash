package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestNewUsesHomeEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnvVar, dir)

	cfg, err := New()

	require.NoError(t, err)
	assert.Equal(t, dir, cfg.BaseDir)
}

func TestNewDefaultsToUserConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnvVar, "")
	t.Setenv("HOME", home)

	cfg, err := New()

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "mysql-stash"), cfg.BaseDir)
}

func TestLoadDBConfigParsesExample(t *testing.T) {
	example, err := os.ReadFile("../config-example.yml")
	require.NoError(t, err)

	cfg := &Config{BaseDir: t.TempDir()}
	require.NoError(t, os.WriteFile(cfg.ConfigFilePath(), example, 0o600))

	dbs, err := cfg.LoadDBConfig()

	require.NoError(t, err)
	assert.Len(t, dbs, 3)
	assert.Equal(t, &DB{Host: "127.0.0.1", Port: 8369, Database: "zigzag", User: "root", Pass: "123"}, dbs["zigzag"])
}

func TestLoadDBConfigMissingFileNamesPath(t *testing.T) {
	cfg := &Config{BaseDir: t.TempDir()}

	_, err := cfg.LoadDBConfig()

	assert.ErrorContains(t, err, cfg.ConfigFilePath())
}

func TestGetStashFilePathDoesNotCreateDirectories(t *testing.T) {
	cfg := &Config{BaseDir: t.TempDir()}

	path, err := cfg.GetStashFilePath("foo", "snap")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cfg.BaseDir, "stashes", "foo", "snap"), path)
	assert.NoDirExists(t, filepath.Dir(path))
}

func TestGetStashFilePathRejectsUnsafeNames(t *testing.T) {
	cfg := &Config{BaseDir: t.TempDir()}

	for _, names := range [][2]string{
		{"foo", "../config.yml"},
		{"..", "snap"},
		{"foo", ""},
		{"foo", ".hidden"},
		{"a/b", "snap"},
	} {
		_, err := cfg.GetStashFilePath(names[0], names[1])
		assert.Error(t, err, "%v", names)
	}
}
