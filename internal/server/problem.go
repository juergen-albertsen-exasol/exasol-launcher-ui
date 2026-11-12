package server

import (
	"encoding/json"
	"net/http"
)

// problem is an RFC 9457 problem detail, as the Administration API returns
// for errors.
type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

func writeProblem(response http.ResponseWriter, status int, detail string) {
	body, err := json.Marshal(problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Detail: detail,
	})
	if err != nil {
		http.Error(response, detail, status)

		return
	}

	response.Header().Set("Content-Type", "application/problem+json")
	response.WriteHeader(status)
	_, _ = response.Write(body)
}

func writeJSON[T any](response http.ResponseWriter, status int, value T) {
	body, err := json.Marshal(value)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write(body)
}
