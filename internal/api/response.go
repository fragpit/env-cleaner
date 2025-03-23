package api

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
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
