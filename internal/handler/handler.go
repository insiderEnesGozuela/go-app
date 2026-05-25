// Package handler is the HTTP delivery layer. Real handlers land in Month 3
// (chi/gin + middlewares). For now we keep only the health endpoint here so
// main.go has a single place to wire routes when the router arrives.
package handler

import (
	"encoding/json"
	"net/http"
)

// Health is a liveness probe. In Hafta 2 we will extend this into a /readyz
// endpoint that pings the database so k8s does not route traffic to a pod
// whose DB pool is exhausted.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
