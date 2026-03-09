package model

import (
	"context"
)

type Connector interface {
	CheckEnvironment(ctx context.Context, env *Environment) error
	DeleteEnvironment(ctx context.Context, env *Environment) error
	GetConnectorType() string
	GetEnvironments(ctx context.Context) ([]Environment, error)
	GetEnvironmentID(ctx context.Context, env *Environment) (string, error)
}
