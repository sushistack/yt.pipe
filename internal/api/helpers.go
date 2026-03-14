package api

import "net/http"

// isHTMX returns true if the request was made by HTMX (has HX-Request header).
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
