package stashers

import (
	"github.com/motomod/mysql-stash/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConnectionArgs(t *testing.T) {
	full := &config.DB{Host: "h", Port: 3307, User: "u", Database: "foo"}
	assert.Equal(t, []string{"-h", "h", "-P", "3307", "-u", "u", "-x", "foo"}, connectionArgs(full, "-x"))

	// Unset options are left to ~/.my.cnf or the client's defaults rather than passed empty.
	assert.Equal(t, []string{"foo"}, connectionArgs(&config.DB{Database: "foo"}))
}

func TestPasswordOnlyPassedWhenSet(t *testing.T) {
	assert.NotContains(t, passwordEnv("", []string{"PATH=/bin"}), "MYSQL_PWD=")
	assert.Contains(t, passwordEnv("secret", []string{"PATH=/bin"}), "MYSQL_PWD=secret")
}
