package postgresql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // Import pq library
	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/utils"
)

type Storage struct {
	DB *sql.DB
}

var _ model.Repository = (*Storage)(nil)

func New(
	host string,
	port int,
	username string,
	password string,
	database string,
) (_ *Storage, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("storage init error: %w", err)
		}
	}()

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, database,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("%w, tx rollback error: %w", err, rbErr)
			}
		}
	}()

	dbCreateQuery := `
	CREATE TABLE IF NOT EXISTS environments (
			env_id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			owner TEXT NOT NULL,
			delete_at TEXT NOT NULL,
			delete_at_sec INT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tokens (
			env_id TEXT PRIMARY KEY,
			token TEXT NOT NULL
	);
	`

	if _, err = tx.Exec(dbCreateQuery); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Storage{
		DB: db,
	}, nil
}

func (s *Storage) WriteEnvironments(
	ctx context.Context,
	envs []model.Environment,
) (err error) {
	return s.executeTransaction(ctx, func(tx *sql.Tx) error {
		q := `INSERT INTO environments (
					env_id,
					type,
					name,
					namespace,
					owner,
					delete_at,
					delete_at_sec
			) VALUES ($1, $2, $3, $4, $5, $6, $7);`

		stmt, err := tx.Prepare(q)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, e := range envs {
			if env, _ := s.GetEnvByID(ctx, e.EnvID); env != nil {
				continue
			}

			log.Infof(
				"New environment added: %s, type: %s, id: %s",
				setName(&e),
				e.Type,
				e.EnvID,
			)

			if _, err := stmt.Exec(
				e.EnvID,
				e.Type,
				e.Name,
				e.Namespace,
				e.Owner,
				e.DeleteAt,
				e.DeleteAtSec,
			); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Storage) GetEnvironments(
	ctx context.Context,
) ([]*model.Environment, error) {
	q := `SELECT * FROM environments;`
	return s.getEnvironments(ctx, q)
}

func (s *Storage) GetEnvByID(
	ctx context.Context,
	id string,
) (*model.Environment, error) {
	q := `SELECT * FROM environments WHERE env_id = $1;`

	row := s.DB.QueryRowContext(ctx, q, id)

	var e model.Environment
	err := row.Scan(
		&e.EnvID, &e.Type, &e.Name, &e.Namespace,
		&e.Owner, &e.DeleteAt, &e.DeleteAtSec,
	)
	if err != nil {
		return nil, fmt.Errorf("get environment by id error: %w", err)
	}

	return &e, nil
}

func (s *Storage) GetStaleEnvironments(
	ctx context.Context,
	tr int64,
) ([]*model.Environment, error) {
	q := `SELECT * FROM environments WHERE delete_at_sec < EXTRACT(EPOCH FROM NOW()) + $1;`

	return s.getEnvironments(ctx, q, tr)
}

func (s *Storage) GetOutdatedEnvironments(
	ctx context.Context,
) ([]*model.Environment, error) {
	q := `SELECT * FROM environments WHERE EXTRACT(EPOCH FROM NOW()) > delete_at_sec;`

	return s.getEnvironments(ctx, q)
}

func (s *Storage) ExtendEnvironment(
	ctx context.Context,
	id, period string,
) (err error) {
	return s.executeTransaction(ctx, func(tx *sql.Tx) error {
		env, err := s.GetEnvByID(ctx, id)
		if err != nil {
			return err
		}

		env.DeleteAt, env.DeleteAtSec, err = utils.IncreaseDeleteAt(
			env.DeleteAt,
			period,
		)
		if err != nil {
			return err
		}

		q := `UPDATE environments SET delete_at = $1, delete_at_sec = $2 WHERE env_id = $3;`

		stmt, err := tx.Prepare(q)
		if err != nil {
			return err
		}
		defer stmt.Close()

		if _, err = stmt.Exec(env.DeleteAt, env.DeleteAtSec, id); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) DeleteEnvironment(ctx context.Context, id string) error {
	return s.executeTransaction(ctx, func(tx *sql.Tx) error {
		q := `DELETE FROM environments WHERE env_id = $1;`

		stmt, err := tx.Prepare(q)
		if err != nil {
			return err
		}
		defer stmt.Close()

		if _, err = stmt.Exec(id); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) SetToken(
	ctx context.Context,
	id string,
) (_ *model.Token, err error) {
	var token string
	err = s.executeTransaction(ctx, func(tx *sql.Tx) error {
		token, err = utils.GenerateToken(16)
		if err != nil {
			return err
		}

		q := `INSERT INTO tokens (env_id, token) VALUES ($1, $2);`

		stmt, err := tx.Prepare(q)
		if err != nil {
			return err
		}
		defer stmt.Close()

		if _, err = stmt.Exec(id, token); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &model.Token{
		EnvID: id,
		Token: token,
	}, nil
}

func (s *Storage) GetToken(
	ctx context.Context,
	id string,
) (*model.Token, error) {
	q := `SELECT token FROM tokens WHERE env_id = $1;`

	rows := s.DB.QueryRowContext(ctx, q, id)

	var token string
	err := rows.Scan(&token)
	if err != nil {
		return nil, fmt.Errorf("get token error: %w", err)
	}

	return &model.Token{
		EnvID: id,
		Token: token,
	}, nil
}

func (s *Storage) DeleteToken(ctx context.Context, id string) error {
	return s.executeTransaction(ctx, func(tx *sql.Tx) error {
		q := `DELETE FROM tokens WHERE env_id = $1;`

		stmt, err := tx.Prepare(q)
		if err != nil {
			return err
		}
		defer stmt.Close()

		if _, err = stmt.Exec(id); err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) executeTransaction(
	ctx context.Context,
	txFunc func(*sql.Tx) error,
) (err error) {
	opts := sql.TxOptions{}
	tx, err := s.DB.BeginTx(ctx, &opts)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("%w, tx rollback error: %w", err, rbErr)
			}
		}
	}()

	if err := txFunc(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Storage) Close() error {
	return s.DB.Close()
}

func setName(env *model.Environment) string {
	name := env.Name
	if env.Namespace != "" {
		name = fmt.Sprintf("%s (namespace: %s)", env.Name, env.Namespace)
	}

	return name
}

func (s *Storage) getEnvironments(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]*model.Environment, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get outdated environments error: %w", err)
	}
	defer rows.Close()

	var envs []*model.Environment
	for rows.Next() {
		var e model.Environment
		err := rows.Scan(
			&e.EnvID,
			&e.Type,
			&e.Name,
			&e.Namespace,
			&e.Owner,
			&e.DeleteAt,
			&e.DeleteAtSec,
		)
		if err != nil {
			return nil, fmt.Errorf("get outdated environments error: %w", err)
		}
		envs = append(envs, &e)
	}

	return envs, nil
}
