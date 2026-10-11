package handler

import (
	"net/http"

	"go.uber.org/zap"
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
