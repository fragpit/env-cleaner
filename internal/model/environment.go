package model

import (
	"context"
	"fmt"
)

type Environment struct {
	EnvID       string
	Type        string
	Name        string
	Namespace   string
	Owner       string
	DeleteAt    string
	DeleteAtSec int64
}

func (e *Environment) DisplayName() string {
	if e.Namespace != "" {
		return fmt.Sprintf(
			"%s (namespace: %s)", e.Name, e.Namespace,
		)
	}
	return e.Name
}

type Repository interface {
	EnvRepository
	TokenRepository
	Close() error
}

type EnvRepository interface {
	WriteEnvironments(ctx context.Context, envs []Environment) error
	GetEnvironments(ctx context.Context) ([]*Environment, error)
	GetEnvByID(ctx context.Context, id string) (*Environment, error)
	GetStaleEnvironments(ctx context.Context, tr int64) ([]*Environment, error)
	GetOutdatedEnvironments(ctx context.Context) ([]*Environment, error)
	ExtendEnvironment(ctx context.Context, id, period string) error
	DeleteEnvironment(ctx context.Context, id string) error
}
