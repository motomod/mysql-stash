package stashers

import (
	"errors"
	"mysql-stash/config"
)

const stashAction = "stashers"
const applyAction = "apply"

const errorUnknownAction = "unknown stashing action"
const errorNoDatabases = "no databases configured"
const errorNoStasher = "no supporting stasher found"

type StasherInterface interface {
	ApplyStash(db *config.DB, dbName string, stashName string) error
	CreateStash(db *config.DB, dbName string, stashName string) error
}

type Stasher struct {
	config   *config.Config
	dbs      map[string]*config.DB
	stashers map[string]StasherInterface
}

func NewStasher(config *config.Config, dbs map[string]*config.DB, stashers map[string]StasherInterface) *Stasher {
	return &Stasher{
		config:   config,
		dbs:      dbs,
		stashers: stashers,
	}
}

func (s Stasher) ApplyStash(dbName string, stashName string) error {
	return s.execute(dbName, stashName, applyAction)
}

func (s Stasher) CreateStash(dbName string, stashName string) error {
	return s.execute(dbName, stashName, stashAction)
}

func (s Stasher) execute(dbName string, stashName string, actionName string) error {
	if actionName != applyAction && actionName != stashAction {
		return errors.New(errorUnknownAction)
	}

	if 0 == len(s.dbs) {
		return errors.New(errorNoDatabases)
	}

	for _, db := range s.dbs {
		stasher, err := s.findStasher(db)

		if err != nil {
			return err
		}

		if applyAction == actionName {
			err = stasher.ApplyStash(db, dbName, stashName)
		}

		if stashAction == actionName {
			err = stasher.CreateStash(db, dbName, stashName)
		}

		if err != nil {
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
