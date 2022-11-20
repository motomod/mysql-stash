package stashers

import (
	"github.com/stretchr/testify/assert"
	config "mysql-stash/config"
	"testing"
)

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

	assert.EqualError(t, err, "no supporting stasher found")
}
