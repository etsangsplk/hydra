package postgres

import (
	"database/sql"
	log "github.com/Sirupsen/logrus"
	"github.com/go-errors/errors"
	"github.com/ory-am/common/pkg"
	"github.com/ory-am/hydra/account"
	"github.com/ory-am/hydra/hash"
)

const accountSchema = `CREATE TABLE IF NOT EXISTS hydra_account (
	id           text NOT NULL PRIMARY KEY,
	username	 text NOT NULL UNIQUE,
	password     text NOT NULL,
	data		 json
)`

type Store struct {
	hasher hash.Hasher
	db     *sql.DB
}

func New(h hash.Hasher, db *sql.DB) *Store {
	return &Store{h, db}
}

func (s *Store) CreateSchemas() error {
	if _, err := s.db.Exec(accountSchema); err != nil {
		log.Warnf("Error creating schema %s: %s", accountSchema, err)
		return errors.New(err)
	}
	return nil
}

func (s *Store) Create(id, username, password, data string) (account.Account, error) {
	// Hash the password
	password, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}

	// Execute SQL statement
	_, err = s.db.Exec("INSERT INTO hydra_account (id, username, password, data) VALUES ($1, $2, $3, $4)", id, username, password, data)
	if err != nil {
		return nil, errors.New(err)
	}

	return &account.DefaultAccount{id, username, password, data}, nil
}

func (s *Store) Get(id string) (account.Account, error) {
	var a account.DefaultAccount
	// Query account
	row := s.db.QueryRow("SELECT id, username, password, data FROM hydra_account WHERE id=$1 LIMIT 1", id)

	// Hydrate struct with data
	if err := row.Scan(&a.ID, &a.Username, &a.Password, &a.Data); err == sql.ErrNoRows {
		return nil, pkg.ErrNotFound
	} else if err != nil {
		return nil, errors.New(err)
	}
	return &a, nil
}

func (s *Store) UpdatePassword(id, oldPassword, newPassword string) (account.Account, error) {
	acc, err := s.authenticateWithIDAndPassword(id, oldPassword)
	if err != nil {
		return nil, err
	}

	// Hash the new password
	newPassword, err = s.hasher.Hash(newPassword)
	if err != nil {
		return nil, err
	}

	// Execute SQL statement
	if _, err = s.db.Exec("UPDATE hydra_account SET (password) = ($2) WHERE id=$1", id, newPassword); err != nil {
		return nil, errors.New(err)
	}

	return &account.DefaultAccount{acc.GetID(), acc.GetUsername(), newPassword, acc.GetData()}, nil
}

func (s *Store) UpdateUsername(id, username, password string) (account.Account, error) {
	acc, err := s.authenticateWithIDAndPassword(id, password)
	if err != nil {
		return nil, err
	}

	// Execute SQL statement
	if _, err = s.db.Exec("UPDATE hydra_account SET (username) = ($2) WHERE id=$1", id, username); err != nil {
		return nil, errors.New(err)
	}

	return &account.DefaultAccount{acc.GetID(), username, acc.GetUsername(), acc.GetData()}, nil
}

func (s *Store) Delete(id string) (err error) {
	if _, err = s.db.Exec("DELETE FROM hydra_account WHERE id=$1", id); err != nil {
		return errors.New(err)
	}
	return nil
}

func (s *Store) Authenticate(username, password string) (account.Account, error) {
	var a account.DefaultAccount
	// Query account
	row := s.db.QueryRow("SELECT id, username, password, data FROM hydra_account WHERE username=$1", username)

	// Hydrate struct with data
	if err := row.Scan(&a.ID, &a.Username, &a.Password, &a.Data); err == sql.ErrNoRows {
		return nil, pkg.ErrNotFound
	} else if err != nil {
		return nil, err
	}

	// Compare the given password with the hashed password stored in the database
	if err := s.hasher.Compare(a.Password, password); err != nil {
		return nil, err
	}

	return &a, nil
}

func (s *Store) UpdateData(id string, data string) (account.Account, error) {
	// Execute SQL statement
	if _, err := s.db.Exec("UPDATE hydra_account SET (data) = ($2) WHERE id=$1", id, data); err != nil {
		return nil, errors.New(err)
	}

	return s.Get(id)
}

func (s *Store) authenticateWithIDAndPassword(id, password string) (account.Account, error) {
	// Look up account
	acc, err := s.Get(id)
	if err != nil {
		return nil, errors.New(err)
	}

	// Compare the given password with the hashed password stored in the database
	if err := s.hasher.Compare(acc.GetPassword(), password); err != nil {
		return nil, errors.New(err)
	}

	return acc, nil
}
