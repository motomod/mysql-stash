package stashers

import (
	"errors"
	"github.com/motomod/mysql-stash/config"
	"sort"
)

const errorNoDatabases = "no databases configured"
const errorNoStasher = "no supporting stasher found"

type StasherInterface interface {
	ApplyStash(db *config.DB, dbName string, stashName string) error
	CreateStash(db *config.DB, dbName string, stashName string) error
}

type Stasher struct {
	dbs      map[string]*config.DB
	stashers map[string]StasherInterface
}

func NewStasher(dbs map[string]*config.DB, stashers map[string]StasherInterface) *Stasher {
	return &Stasher{
		dbs:      dbs,
		stashers: stashers,
	}
}

func (s Stasher) ApplyStash(stashName string) error {
	return s.forEach(func(stasher StasherInterface, db *config.DB, dbName string) error {
		return stasher.ApplyStash(db, dbName, stashName)
	})
}

func (s Stasher) CreateStash(stashName string) error {
	return s.forEach(func(stasher StasherInterface, db *config.DB, dbName string) error {
		return stasher.CreateStash(db, dbName, stashName)
	})
}

// forEach runs fn against every configured database, in name order, stopping at the first error.
func (s Stasher) forEach(fn func(stasher StasherInterface, db *config.DB, dbName string) error) error {
	if 0 == len(s.dbs) {
		return errors.New(errorNoDatabases)
	}

	dbNames := make([]string, 0, len(s.dbs))

	for dbName := range s.dbs {
		dbNames = append(dbNames, dbName)
	}

	sort.Strings(dbNames)

	for _, dbName := range dbNames {
		db := s.dbs[dbName]
		stasher, err := s.findStasher(db)

		if err != nil {
			return err
		}

		if err = fn(stasher, db, dbName); err != nil {
			return err
		}
	}

	return nil
}

func (s Stasher) findStasher(*config.DB) (StasherInterface, error) {
	if stasher, ok := s.stashers["mysql"]; ok {
		return stasher, nil
	}

	return nil, errors.New(errorNoStasher)
}
