package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

// NewHandler builds the API mux. Routes:
//
//	GET /healthz                                        liveness, no auth
//	GET /api/v1/repos                                   tracked repos
//	GET /api/v1/repos/{owner}/{name}                    one repo
//	GET /api/v1/repos/{owner}/{name}/metrics            Report for ?from=&to=
//	GET /api/v1/repos/{owner}/{name}/metrics/monthly    one Report per month
//
// from / to are YYYY-MM with the same defaults as `devpulse metrics`:
// from is the current month, to (exclusive) is from + 1 month.
func NewHandler(cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	h := &handlers{cfg: cfg}

	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/repos", h.listRepos)
	api.HandleFunc("GET /api/v1/repos/{owner}/{name}", h.getRepo)
	api.HandleFunc("GET /api/v1/repos/{owner}/{name}/metrics", h.getMetrics)
	api.HandleFunc("GET /api/v1/repos/{owner}/{name}/metrics/monthly", h.getMonthlyMetrics)
	api.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("/api/", requireToken(cfg.Token, api))
	return mux
}

// requireToken enforces `Authorization: Bearer <token>`. An empty
// configured token disables the check; CheckBind only allows that on a
// loopback address.
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="devpulse"`)
				writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type handlers struct {
	cfg Config
}

// repoJSON is the API shape of a tracked repository.
type repoJSON struct {
	ID            string  `json:"id"`
	FullName      string  `json:"full_name"`
	Owner         string  `json:"owner"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	Description   *string `json:"description"`
	DefaultBranch string  `json:"default_branch"`
	Disabled      bool    `json:"disabled"`
}

func toRepoJSON(r repo.Repo) repoJSON {
	return repoJSON{
		ID:            r.ID,
		FullName:      r.Name.String(),
		Owner:         r.Name.Owner,
		Name:          r.Name.Name,
		Provider:      r.Provider,
		Description:   r.Description,
		DefaultBranch: r.DefaultBranch,
		Disabled:      r.Disabled,
	}
}

// monthlyJSON wraps the per-month reports of a trend request.
type monthlyJSON struct {
	Repo   string           `json:"repo"`
	From   string           `json:"from"`
	To     string           `json:"to"`
	Months []metrics.Report `json:"months"`
}

func (h *handlers) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.cfg.Repos.ListAll(r.Context())
	if err != nil {
		h.internalError(w, "list repos", err)
		return
	}
	out := make([]repoJSON, 0, len(repos))
	for _, rp := range repos {
		out = append(out, toRepoJSON(rp))
	}
	writeJSON(w, http.StatusOK, map[string][]repoJSON{"repos": out})
}

func (h *handlers) getRepo(w http.ResponseWriter, r *http.Request) {
	rp, ok := h.findRepo(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toRepoJSON(rp))
}

func (h *handlers) getMetrics(w http.ResponseWriter, r *http.Request) {
	rp, ok := h.findRepo(w, r)
	if !ok {
		return
	}
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	report, err := metrics.Compute(r.Context(), h.cfg.Metrics, rp, win, h.cfg.Now())
	if err != nil {
		h.internalError(w, "compute metrics", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *handlers) getMonthlyMetrics(w http.ResponseWriter, r *http.Request) {
	rp, ok := h.findRepo(w, r)
	if !ok {
		return
	}
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	if err := win.CheckTrend(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	months, err := metrics.ComputeMonthly(r.Context(), h.cfg.Metrics, rp, win, h.cfg.Now())
	if err != nil {
		h.internalError(w, "compute monthly metrics", err)
		return
	}
	writeJSON(w, http.StatusOK, monthlyJSON{
		Repo:   rp.Name.String(),
		From:   win.From.Format("2006-01"),
		To:     win.To.Format("2006-01"),
		Months: months,
	})
}

// findRepo resolves {owner}/{name}, answering 400 for a malformed name
// and 404 for a repo that is not tracked.
func (h *handlers) findRepo(w http.ResponseWriter, r *http.Request) (repo.Repo, bool) {
	name, err := repo.ParseFullName(r.PathValue("owner") + "/" + r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return repo.Repo{}, false
	}
	rp, err := h.cfg.Repos.FindByFullName(r.Context(), "github", name)
	if errors.Is(err, persistence.ErrRepoNotFound) {
		writeError(w, http.StatusNotFound, "repo "+name.String()+" is not tracked")
		return repo.Repo{}, false
	}
	if err != nil {
		h.internalError(w, "find repo", err)
		return repo.Repo{}, false
	}
	return rp, true
}

func (h *handlers) window(w http.ResponseWriter, r *http.Request) (metrics.Window, bool) {
	q := r.URL.Query()
	win, err := metrics.ParseWindow(q.Get("from"), q.Get("to"), h.cfg.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return metrics.Window{}, false
	}
	return win, true
}

// internalError logs the cause and answers a generic 500, so database
// details never leak to the client.
func (h *handlers) internalError(w http.ResponseWriter, op string, err error) {
	h.cfg.Logger.Error("api: "+op, slog.String("err", err.Error()))
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
