package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
	"github.com/oti-adjei/zook/internal/webhook"
	"github.com/oti-adjei/zook/internal/wire"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Deployer is the slice of the release engine the server drives. *core.Engine
// satisfies it.
type Deployer interface {
	Deploy(ctx context.Context, s config.Stack, version string) error
	Rollback(ctx context.Context, s config.Stack) error
}

// EngineFactory builds a Deployer for a stack.
type EngineFactory func(s config.Stack) Deployer

// Options configures a Server.
type Options struct {
	StacksRoot    string        // where stacks live (required)
	APIToken      string        // bearer token for mutating endpoints; "" disables them
	WebhookSecret []byte        // GitHub webhook HMAC secret; nil rejects all deliveries
	NewEngine     EngineFactory // defaults to the real wire-built engine
	Log           *log.Logger   // defaults to log.Default()
}

// Server serves zook's HTTP API. It is a transport over core.Engine: deploys
// triggered here produce the same logs, state, and rollback behavior as CLI
// deploys. The engine assumes single-writer; Jobs enforces it per stack.
type Server struct {
	opts          Options
	jobs          *Jobs
	defaultEngine EngineFactory
}

// New returns a Server. Call Handler() to mount it.
func New(o Options) *Server {
	if o.NewEngine == nil {
		o.NewEngine = func(s config.Stack) Deployer {
			return wire.EngineFor(s, wire.DefaultTimeout())
		}
	}
	if o.Log == nil {
		o.Log = log.Default()
	}
	return &Server{opts: o, jobs: NewJobs(), defaultEngine: o.NewEngine}
}

// Handler returns the fully-routed HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /stacks", s.handleListStacks)
	mux.HandleFunc("GET /stacks/{name}", s.handleGetStack)
	mux.HandleFunc("POST /deploy", s.requireAuth(s.handleDeploy))
	mux.HandleFunc("POST /rollback", s.requireAuth(s.handleRollback))
	mux.HandleFunc("GET /jobs/{id}", s.handleGetJob)
	mux.HandleFunc("POST /hooks/github", s.handleGitHubWebhook)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// requireAuth guards mutating endpoints with the bearer token. An unset
// token fails closed: the endpoint is disabled (503), never open.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opts.APIToken == "" {
			writeError(w, http.StatusServiceUnavailable, "api token not configured — mutating endpoints disabled (set ZOOK_API_TOKEN)")
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, prefix)), []byte(s.opts.APIToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := config.DiscoverStacks(s.opts.StacksRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type stackView struct {
		Name       string `json:"name"`
		Runtime    string `json:"runtime"`
		HasConfig  bool   `json:"configured"`
		Current    string `json:"current"`
		Previous   string `json:"previous,omitempty"`
		LastResult string `json:"last_result,omitempty"`
	}
	out := make([]stackView, 0, len(stacks))
	for _, st := range stacks {
		state, _ := core.LoadState(st.Dir)
		var last string
		if n := len(state.History); n > 0 {
			last = string(state.History[n-1].Result)
		}
		out = append(out, stackView{
			Name:       st.Name,
			Runtime:    st.Runtime,
			HasConfig:  st.Config != nil,
			Current:    state.Current,
			Previous:   state.Previous,
			LastResult: last,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"stacks": out})
}

func (s *Server) handleGetStack(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	stack, err := config.FindStack(s.opts.StacksRoot, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	state, err := core.LoadState(stack.Dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    stack.Name,
		"runtime": stack.Runtime,
		"state":   state,
	})
}

func (s *Server) findStack(w http.ResponseWriter, name string) (config.Stack, bool) {
	stack, err := config.FindStack(s.opts.StacksRoot, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return config.Stack{}, false
	}
	return stack, true
}

type deployRequest struct {
	Stack   string `json:"stack"`
	Version string `json:"version"`
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var req deployRequest
	if err := decodeBody(w, r, &req); err != nil {
		return // decodeBody already responded
	}
	if req.Stack == "" || req.Version == "" {
		writeError(w, http.StatusBadRequest, "both 'stack' and 'version' are required")
		return
	}
	stack, ok := s.findStack(w, req.Stack)
	if !ok {
		return
	}
	s.startJob(w, "deploy", stack, req.Version)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req deployRequest
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	if req.Stack == "" {
		writeError(w, http.StatusBadRequest, "'stack' is required")
		return
	}
	stack, ok := s.findStack(w, req.Stack)
	if !ok {
		return
	}
	s.startJob(w, "rollback", stack, "")
}

// startJob enqueues a deploy or rollback, mapping ErrStackBusy to 409.
func (s *Server) startJob(w http.ResponseWriter, kind string, stack config.Stack, version string) {
	job, err := s.jobs.Start(kind, stack.Name, version, func(ctx context.Context) error {
		if kind == "rollback" {
			return s.opts.NewEngine(stack).Rollback(ctx, stack)
		}
		return s.opts.NewEngine(stack).Deploy(ctx, stack, version)
	})
	if err != nil {
		if errors.Is(err, ErrStackBusy) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.opts.Log.Printf("%s %s queued (job %s)", kind, stack.Name, job.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": job.ID, "status": string(job.Status)})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r)
	if err != nil {
		return // readBody already responded
	}
	// Fail closed: no secret configured → reject every delivery.
	if len(s.opts.WebhookSecret) == 0 {
		writeError(w, http.StatusUnauthorized, "webhook secret not configured (set ZOOK_WEBHOOK_SECRET)")
		return
	}
	if !webhook.Verify(s.opts.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}
	switch r.Header.Get("X-GitHub-Event") {
	case "ping":
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
	case "push":
		event, err := webhook.ParsePush(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		stacks, err := config.DiscoverStacks(s.opts.StacksRoot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, stack := range stacks {
			if stack.Config == nil || stack.Config.DeployOn == nil || stack.Config.DeployOn.GitHub == nil {
				continue
			}
			if version, ok := event.Match(*stack.Config.DeployOn.GitHub); ok {
				s.opts.Log.Printf("webhook: %s push → deploying %s %s", event.Repo, stack.Name, version)
				s.startJob(w, "deploy", stack, version)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	}
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "reading body: "+err.Error())
		return nil, err
	}
	if len(body) > maxBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "body too large")
		return nil, errors.New("body too large")
	}
	return body, nil
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	body, err := readBody(w, r)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return err
	}
	return nil
}
