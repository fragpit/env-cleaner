package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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
		slog.Error("error encoding JSON response", slog.Any("error", err))
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
		slog.Error("error encoding JSON response", slog.Any("error", err))
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
		slog.Error(
			"validation error",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		sendErrorResponse(w, http.StatusBadRequest, ve.Msg)
	case errors.As(err, &nf):
		slog.Error(
			"not found",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		sendErrorResponse(w, http.StatusNotFound, nf.Msg)
	case errors.As(err, &ce):
		slog.Warn(
			"conflict",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		sendErrorResponse(w, http.StatusConflict, ce.Msg)
	default:
		slog.Error(
			"internal error",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		sendErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	}
}
