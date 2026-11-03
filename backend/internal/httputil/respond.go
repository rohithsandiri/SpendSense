package httputil

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// Error writes a {"message": ...} body, matching the original Express API.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"message": message})
}

// Decode reads a JSON request body into dst.
func Decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
