package stashers

import (
	"errors"
	"log"
	"mysql-stash/config"
)

const stashAction = "stashers"
const applyAction = "apply"

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
		return errors.New("unknown stashing action")
	}

	for _, db := range s.dbs {
		stasher, err := s.findStasher(db)

		log.Println(stasher, err)

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

// FindStasher Currently only support mysql stasher based on our config files
func (s Stasher) findStasher(*config.DB) (StasherInterface, error) {
	if stasher, ok := s.stashers["mysql"]; ok {
		return stasher, nil
	}

	return nil, errors.New("no supporting stasher found")
}
