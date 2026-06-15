// Package handler is the HTTP delivery layer. Real handlers land in Month 3
// (chi/gin + middlewares). For now we keep only the health/readiness endpoints
// here so main.go has a single place to wire routes when the router arrives.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger is the minimal surface a readiness check needs from a datastore.
// *pgxpool.Pool satisfies it, but depending on this tiny interface (not the
// concrete pool) keeps the handler package free of any database import.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health is a liveness probe: "is the process up and serving?" It must NOT
// touch dependencies — a liveness check that pings the DB would make k8s kill a
// perfectly healthy pod during a transient DB blip. Liveness = process alive,
// readiness = ready for traffic. They are different questions.
func Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness returns a probe that reports whether the service can serve traffic,
// i.e. its database is reachable. k8s routes traffic only to Ready pods, so if
// the DB pool is exhausted/unreachable this returns 503 and the pod is pulled
// out of the load balancer until it recovers — no requests hit a dead backend.
func Readiness(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bound the probe: a hanging DB must not hang the probe too. 2s is well
		// under typical kubelet probe timeouts.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"reason": "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
