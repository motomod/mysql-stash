package stashers

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	config "mysql-stash/config"
	"testing"
)

func testDB(name string) *config.DB {
	return &config.DB{
		Host:     "test",
		Port:     0,
		Database: name,
		User:     "test",
		Pass:     "test",
	}
}

func TestNoDbsInConfigReturnsError(t *testing.T) {
	stasher := NewStasher(map[string]*config.DB{}, map[string]StasherInterface{})

	err := stasher.ApplyStash("test")

	assert.EqualError(t, err, errorNoDatabases)
}

func TestMissingMySQLStasherReturnsError(t *testing.T) {
	dbs := map[string]*config.DB{"foo": testDB("foo")}
	stasher := NewStasher(dbs, map[string]StasherInterface{})

	err := stasher.ApplyStash("test")

	assert.EqualError(t, err, errorNoStasher)
}

type call struct {
	action    string
	dbName    string
	database  string
	stashName string
}

type mockStasher struct {
	calls []call
	err   error
}

func (m *mockStasher) ApplyStash(db *config.DB, dbName string, stashName string) error {
	m.calls = append(m.calls, call{"apply", dbName, db.Database, stashName})

	return m.err
}

func (m *mockStasher) CreateStash(db *config.DB, dbName string, stashName string) error {
	m.calls = append(m.calls, call{"stash", dbName, db.Database, stashName})

	return m.err
}

func TestEachDatabaseIsStashedUnderItsOwnName(t *testing.T) {
	dbs := map[string]*config.DB{
		"foo": testDB("foo_db"),
		"bar": testDB("bar_db"),
	}
	mock := &mockStasher{}
	stasher := NewStasher(dbs, map[string]StasherInterface{"mysql": mock})

	require.NoError(t, stasher.CreateStash("snap"))
	require.NoError(t, stasher.ApplyStash("snap"))

	assert.Equal(t, []call{
		{"stash", "bar", "bar_db", "snap"},
		{"stash", "foo", "foo_db", "snap"},
		{"apply", "bar", "bar_db", "snap"},
		{"apply", "foo", "foo_db", "snap"},
	}, mock.calls)
}

func TestStopsAtFirstError(t *testing.T) {
	dbs := map[string]*config.DB{
		"foo": testDB("foo_db"),
		"bar": testDB("bar_db"),
	}
	mock := &mockStasher{err: errors.New("boom")}
	stasher := NewStasher(dbs, map[string]StasherInterface{"mysql": mock})

	assert.EqualError(t, stasher.CreateStash("snap"), "boom")
	assert.Len(t, mock.calls, 1)
}
