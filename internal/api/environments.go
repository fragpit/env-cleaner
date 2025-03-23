package api

import (
	"context"
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/utils"
)

type Environment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Owner     string `json:"owner"`
	Type      string `json:"type"`
	TTL       string `json:"ttl"`
}

// TODO: handle context in api.* package
func (a *API) getEnvironments(w http.ResponseWriter, _ *http.Request) {
	ctx := context.TODO()
	envs, err := a.Repository.GetEnvironments(ctx)
	if err != nil {
		log.Errorf("Error getting environments: %v", err)
		sendErrorResponse(
			w,
			http.StatusInternalServerError,
			"error getting environments",
		)
		return
	}

	sendSuccessResponse(w, envs)
}

func (a *API) addEnvironment(w http.ResponseWriter, r *http.Request) {
	var env Environment
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		log.Errorf("Error decoding request: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error decoding request")
		return
	}

	conn, err := a.ConnectorFactory.GetConnector(env.Type)
	if err != nil {
		log.Errorf("Error getting connector: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error getting connector")
		return
	}

	deleteAt, deleteAtSec, err := utils.SetDeleteAt(env.TTL)
	if err != nil {
		log.Errorf("Error setting delete ttl: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error setting delete ttl")
		return
	}

	envModel := &model.Environment{
		Type:        env.Type,
		Name:        env.Name,
		Namespace:   env.Namespace,
		Owner:       env.Owner,
		DeleteAt:    deleteAt,
		DeleteAtSec: deleteAtSec,
	}

	// TODO: handle context in api.* package
	ctx := context.TODO()

	envModel.EnvID, err = conn.GetEnvironmentID(ctx, envModel)
	if err != nil {
		log.Errorf("Error getting environment id: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error getting environment id")
		return
	}

	if err := conn.CheckEnvironment(ctx, envModel); err != nil {
		log.Errorf("Error checking environment: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error checking environment")
		return
	}

	if _, err := a.Repository.GetEnvByID(ctx, envModel.EnvID); err == nil {
		log.Warnf("Environment already exists: %s", envModel.EnvID)
		sendErrorResponse(w, http.StatusConflict, "environment already exists")
		return
	}

	envs := []model.Environment{*envModel}
	if err := a.Repository.WriteEnvironments(ctx, envs); err != nil {
		log.Errorf("Error writing environments: %v", err)
		sendErrorResponse(
			w,
			http.StatusInternalServerError,
			"error writing environments",
		)
		return
	}

	sendSuccessResponse(w, envModel)
}
