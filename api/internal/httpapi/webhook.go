package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Mustenaka/astro-go-blog/api/internal/build"
)

// POST /hooks/github — GitHub push webhook. Only pushes to MAIN_BRANCH trigger a build.
func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GitHubWebhookSecret == "" {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	if !VerifyGitHubSignature(s.cfg.GitHubWebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event": "ping"})
		return
	}
	if event != "push" {
		writeJSON(w, http.StatusOK, map[string]any{"ignored": true, "reason": "event is not push"})
		return
	}
	var payload struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if payload.Ref != "refs/heads/"+s.cfg.MainBranch {
		writeJSON(w, http.StatusOK, map[string]any{"ignored": true, "reason": "not the main branch", "ref": payload.Ref})
		return
	}
	job, err := s.builds.Trigger(r.Context(), "webhook")
	if errors.Is(err, build.ErrBusy) {
		writeJSON(w, http.StatusOK, map[string]any{"ignored": true, "reason": err.Error()})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// VerifyGitHubSignature checks the `sha256=<hex>` header GitHub sends for a shared secret.
func VerifyGitHubSignature(secret string, body []byte, header string) bool {
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(strings.TrimPrefix(header, "sha256=")))
}
