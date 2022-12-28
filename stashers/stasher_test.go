package stashers

import (
	"github.com/stretchr/testify/assert"
	config "mysql-stash/config"
	"testing"
)

func TestNoDbsInConfigReturnsError(t *testing.T) {
	cfg := config.New()
	dbs := map[string]*config.DB{}
	stashers := map[string]StasherInterface{}

	stasher := NewStasher(&cfg, dbs, stashers)

	err := stasher.ApplyStash("test", "test")

	assert.EqualError(t, err, errorNoDatabases)
}

func TestMissingMySQLStasherReturnsError(t *testing.T) {
	cfg := config.New()
	dbs := map[string]*config.DB{
		"poop": {
			Host:     "test",
			Port:     0,
			Database: "test",
			User:     "test",
			Pass:     "test",
		},
	}
	stashers := map[string]StasherInterface{}

	stasher := NewStasher(&cfg, dbs, stashers)

	err := stasher.ApplyStash("test", "test")

	assert.EqualError(t, err, errorNoStasher)
}

type mockStasher struct {
	Counter int
}

func (m *mockStasher) ApplyStash(db *config.DB, dbName string, stashName string) error {
	m.Counter++

	return nil
}

func (m *mockStasher) CreateStash(db *config.DB, dbName string, stashName string) error {
	m.Counter++

	return nil
}

func TestMysqlStasherIsFoundAndExecuted(t *testing.T) {
	cfg := config.New()
	dbs := map[string]*config.DB{
		"poop": {
			Host:     "test",
			Port:     0,
			Database: "test",
			User:     "test",
			Pass:     "test",
		},
	}

	counter := 0

	mock := &mockStasher{counter}

	stashers := map[string]StasherInterface{
		"mysql": mock,
	}

	stasher := NewStasher(&cfg, dbs, stashers)

	stasher.ApplyStash("test", "test")
	stasher.CreateStash("test", "test")

	assert.Equal(t, 2, mock.Counter)
}
