package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/MaxPa1/Gofermart/internal/model"
)

const maxJSONBodySize = 4 << 10

var errTrailingData = errors.New("unexpected data after JSON object")

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	req, err := readJSON[T](http.MaxBytesReader(w, r.Body, maxJSONBodySize))
	if err != nil {
		writeDecodeError(w, err)
		var zero T
		return zero, false
	}
	return req, true
}

func readJSON[T any](body io.Reader) (T, error) {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()

	var v T
	if err := dec.Decode(&v); err != nil {
		return v, err
	}

	err := dec.Decode(&struct{}{})
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.Is(err, io.EOF):
		return v, nil
	case errors.As(err, &maxBytesErr):
		return v, err
	default:
		return v, errTrailingData
	}
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr):
		writeError(w, http.StatusRequestEntityTooLarge, "")
	case errors.Is(err, model.ErrInvalidAmount):
		writeError(w, http.StatusBadRequest, "sum must be a number with at most two decimal places")
	default:
		writeError(w, http.StatusBadRequest, "invalid JSON body")
	}
}
