package service

import (
	"fmt"

	"github.com/fragpit/env-cleaner/internal/model"
)

type ConnectorFactory interface {
	GetConnector(connType string) (model.Connector, error)
}

type ConnectorList struct {
	Connectors map[string]model.Connector
}

func (f *ConnectorList) GetConnector(
	connType string,
) (model.Connector, error) {
	connector, ok := f.Connectors[connType]
	if !ok {
		return nil, fmt.Errorf(
			"connector not found for type: %s",
			connType,
		)
	}

	return connector, nil
}
