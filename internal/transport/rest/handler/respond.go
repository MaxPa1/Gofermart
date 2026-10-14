package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	if msg == "" {
		msg = http.StatusText(status)
	}
	http.Error(w, msg, status)
}

func writeInternalError(w http.ResponseWriter, r *http.Request, logger *zap.Logger, op string, err error) {
	logger.Error("request failed",
		zap.String("op", op),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Error(err),
	)
	writeError(w, http.StatusInternalServerError, "")
}

func writeJSON(w http.ResponseWriter, r *http.Request, logger *zap.Logger, op string, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeInternalError(w, r, logger, op, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func userIDFromRequest(w http.ResponseWriter, r *http.Request, logger *zap.Logger, op string) (int64, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeInternalError(w, r, logger, op, errors.New("user id is missing in request context"))
		return 0, false
	}
	return userID, true
}
