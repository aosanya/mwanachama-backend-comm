package routes

import (
	"encoding/json"
	"net/http"
)

// writeJSON and writeErr mirror the gateway's own internal/api/http/wire.go
// byte-for-byte on purpose: a caller of a route built from this package
// must not be able to tell, from the response shape, that it moved here.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes {"error": msg}.
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// readJSON decodes a JSON body into v, refusing unknown fields the same way
// the gateway's readJSON does.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
