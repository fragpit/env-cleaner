package api

import (
	"encoding/base64"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverAPIKey := a.Config.AdminAPIKey

		authHeader := r.Header.Get("Authorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		encodedKey := strings.TrimPrefix(authHeader, "Basic ")
		decodedKeyBytes, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			log.Errorf("Error decoding base64: %v", err)
			sendErrorResponse(w, http.StatusBadRequest, "Invalid base64 encoding")
			return
		}

		if string(decodedKeyBytes) != serverAPIKey {
			log.Errorf("Invalid API key: %s", string(decodedKeyBytes))
			sendErrorResponse(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
