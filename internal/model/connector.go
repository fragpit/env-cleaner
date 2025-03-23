package model

import (
	"context"
	"fmt"
)

type Connector interface {
	CheckEnvironment(ctx context.Context, env *Environment) error
	DeleteEnvironment(ctx context.Context, env *Environment) error
	GetConnectorType() string
	GetEnvironments(ctx context.Context) ([]Environment, error)
	GetEnvironmentID(ctx context.Context, env *Environment) (string, error)
}

type ConnectorFactory interface {
	GetConnector(connType string) (Connector, error)
}

type ConnectorList struct {
	Connectors map[string]Connector
}

func (f *ConnectorList) GetConnector(connType string) (Connector, error) {
	connector, ok := f.Connectors[connType]
	if !ok {
		return nil, fmt.Errorf("connector not found for type: %s", connType)
	}

	return connector, nil
}
