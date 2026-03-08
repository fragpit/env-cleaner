package api

import "github.com/fragpit/env-cleaner/internal/model"

// EnvironmentRequest is a DTO for creating an environment.
type EnvironmentRequest struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Owner     string `json:"owner"`
	Type      string `json:"type"`
	TTL       string `json:"ttl"`
}

// ToModel converts EnvironmentRequest DTO to domain model.
func (r *EnvironmentRequest) ToModel() *model.Environment {
	return &model.Environment{
		Type:      r.Type,
		Name:      r.Name,
		Namespace: r.Namespace,
		Owner:     r.Owner,
	}
}

// EnvironmentResponse is a DTO for returning environment data.
type EnvironmentResponse struct {
	EnvID     string `json:"env_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Owner     string `json:"owner"`
	DeleteAt  string `json:"delete_at,omitempty"`
}

// NewEnvironmentResponse converts domain model to response DTO.
func NewEnvironmentResponse(
	e *model.Environment,
) *EnvironmentResponse {
	return &EnvironmentResponse{
		EnvID:     e.EnvID,
		Type:      e.Type,
		Name:      e.Name,
		Namespace: e.Namespace,
		Owner:     e.Owner,
		DeleteAt:  e.DeleteAt,
	}
}

// NewEnvironmentListResponse converts a slice of domain models
// to a slice of response DTOs.
func NewEnvironmentListResponse(
	envs []*model.Environment,
) []*EnvironmentResponse {
	result := make([]*EnvironmentResponse, len(envs))
	for i, e := range envs {
		result[i] = NewEnvironmentResponse(e)
	}
	return result
}
