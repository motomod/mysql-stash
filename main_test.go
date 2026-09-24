package main

import (
	"github.com/motomod/mysql-stash/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetDBsFromArgument(t *testing.T) {
	foo := &config.DB{Database: "foo"}
	bar := &config.DB{Database: "bar"}
	databases := map[string]*config.DB{"foo": foo, "bar": bar}

	dbs, err := getDBsFromArgument("all", databases)
	require.NoError(t, err)
	assert.Equal(t, databases, dbs)

	dbs, err = getDBsFromArgument("foo", databases)
	require.NoError(t, err)
	assert.Equal(t, map[string]*config.DB{"foo": foo}, dbs)

	_, err = getDBsFromArgument("nope", databases)
	assert.EqualError(t, err, "db 'nope' doesn't exist in config")
}

func TestRunRejectsBadArguments(t *testing.T) {
	t.Setenv(config.HomeEnvVar, t.TempDir())

	assert.ErrorContains(t, run(nil), "usage:")
	assert.ErrorContains(t, run([]string{"bogus"}), "unrecognised command 'bogus'")
	assert.ErrorContains(t, run([]string{"stash", "foo"}), "needs a database name and a stash name")
	assert.ErrorContains(t, run([]string{"stash", "foo", "snap"}), "no config found")
}
