package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
)

type SecretProvider interface {
	GetSecrets(project string) (map[string]string, error)
	LogAccess(appName, processPath, keysRequested string, pid int) error
}

type ConfigProvider interface {
	HasApp(appName string) bool
	AllowedPaths(appName string) []string
}

type healthResponse struct {
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

type secretsResponse struct {
	App     string            `json:"app"`
	Secrets map[string]string `json:"secrets"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (d *Daemon) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "running",
		PID:    os.Getpid(),
	})
}

// secretsHandler handles requests for secrets. It performs authentication and authorization
// based on the peer credentials and the daemon config, then returns the requested secrets if authorized.
// The app name must be provided as a query parameter (e.g. /secrets?app=myapp).
func (d *Daemon) secretsHandler(w http.ResponseWriter, r *http.Request) {
	appName := r.URL.Query().Get("app")
	if appName == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing 'app' query parameter"})
		return
	}

	cred, ok := credFromContext(r.Context())
	if !ok || cred == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unable to authenticate"})
		return
	}

	if cred.UID != os.Getuid() {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "uid mismatch"})
		return
	}

	currentStartTime, err := readProcStartTime(cred.PID)
	if err != nil || currentStartTime != cred.StartTime {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "pid has been reused or cannot be verified"})
		return
	}

	if !d.config.HasApp(appName) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: fmt.Sprintf("app %q not found in config", appName)})
		return
	}

	allowedPaths := d.config.AllowedPaths(appName)
	if len(allowedPaths) > 0 {
		exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", cred.PID))
		if err != nil {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "cannot verify process path"})
			return
		}
		if !matchesPath(exe, allowedPaths) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "process not in allowed paths"})
			return
		}
	}

	secrets, err := d.vault.GetSecrets(appName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}

	exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", cred.PID))
	d.vault.LogAccess(appName, exe, strings.Join(keys, ","), cred.PID)

	writeJSON(w, http.StatusOK, secretsResponse{
		App:     appName,
		Secrets: secrets,
	})
}

func matchesPath(exe string, allowed []string) bool {
	return slices.Contains(allowed, exe)
}
