package api

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/model"
)

type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func sendSuccessResponse(w http.ResponseWriter, data any) {
	response := Response{
		Success: true,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("error encoding JSON response: %v", err)
		sendErrorResponse(
			w,
			http.StatusInternalServerError,
			"Internal server error",
		)
	}
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := Response{
		Success: false,
		Error: &Error{
			Code:    statusCode,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("error encoding JSON response: %v", err)
	}
}

func handleServiceError(
	w http.ResponseWriter, err error, subject string,
) {
	var ve *model.ValidationError
	var nf *model.NotFoundError
	var ce *model.ConflictError

	switch {
	case errors.As(err, &ve):
		log.Errorf("Validation error [%s]: %v", subject, err)
		sendErrorResponse(w, http.StatusBadRequest, ve.Msg)
	case errors.As(err, &nf):
		log.Errorf("Not found [%s]: %v", subject, err)
		sendErrorResponse(w, http.StatusNotFound, nf.Msg)
	case errors.As(err, &ce):
		log.Warnf("Conflict [%s]: %v", subject, err)
		sendErrorResponse(w, http.StatusConflict, ce.Msg)
	default:
		log.Errorf("Internal error [%s]: %v", subject, err)
		sendErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	}
}
