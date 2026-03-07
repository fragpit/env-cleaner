package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/model"
)

type Environment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Owner     string `json:"owner"`
	Type      string `json:"type"`
	TTL       string `json:"ttl"`
}

type EnvironmentHandler struct {
	service EnvironmentService
}

func NewEnvironmentHandler(svc EnvironmentService) *EnvironmentHandler {
	return &EnvironmentHandler{service: svc}
}

func (h *EnvironmentHandler) GetEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := h.service.GetEnvironments(r.Context())
	if err != nil {
		handleServiceError(w, err, "get environments")
		return
	}

	sendSuccessResponse(w, envs)
}

func (h *EnvironmentHandler) AddEnvironment(w http.ResponseWriter, r *http.Request) {
	var env Environment
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		log.Errorf("Error decoding request: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "error decoding request")
		return
	}

	envModel := &model.Environment{
		Type:      env.Type,
		Name:      env.Name,
		Namespace: env.Namespace,
		Owner:     env.Owner,
	}

	if err := h.service.AddEnvironment(r.Context(), envModel, env.TTL); err != nil {
		handleServiceError(w, err, envModel.Name)
		return
	}

	sendSuccessResponse(w, envModel)
}

func (h *EnvironmentHandler) ExtendEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryParams := r.URL.Query()
	envID := queryParams.Get("env_id")
	period := queryParams.Get("period")
	token := queryParams.Get("token")

	env, err := h.service.ExtendEnvironment(ctx, envID, period, token)
	if err != nil {
		handleServiceError(w, err, envID)
		return
	}

	log.Infof(
		"Extended environment: %s, type: %s, id: %s, period: %s, token: %s",
		env.DisplayName(), env.Type, env.EnvID, period, token,
	)

	msg := fmt.Sprintf("Extended environment: %s, type: %s, period: %s",
		env.DisplayName(), env.Type, period)
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, msg); err != nil {
		log.Errorf("Error writing response, environment id: %s, %v", envID, err)
	}
}

