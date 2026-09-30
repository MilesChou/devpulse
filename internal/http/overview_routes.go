package http

import (
	"net/http"

	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/repo"
)

// registerOverviewRoutes adds the cross-repo endpoints: the report and
// trend across every tracked, enabled repo, and the comparison rows of
// the dashboard's Overview page.
func registerOverviewRoutes(api *http.ServeMux, h *handlers) {
	api.HandleFunc("GET /api/v1/metrics", h.getAllMetrics)
	api.HandleFunc("GET /api/v1/metrics/monthly", h.getAllMonthlyMetrics)
	api.HandleFunc("GET /api/v1/overview/repos", h.getRepoOverview)
	api.HandleFunc("GET /api/v1/overview/members", h.getMemberOverview)
}

// enabledRepos lists the repos cross-repo metrics cover: tracked and
// not disabled, as `devpulse sync` skips disabled ones.
func (h *handlers) enabledRepos(w http.ResponseWriter, r *http.Request) ([]repo.Repo, bool) {
	all, err := h.cfg.Repos.ListAll(r.Context())
	if err != nil {
		h.internalError(w, "list repos", err)
		return nil, false
	}
	out := make([]repo.Repo, 0, len(all))
	for _, rp := range all {
		if !rp.Disabled {
			out = append(out, rp)
		}
	}
	return out, true
}

func (h *handlers) getAllMetrics(w http.ResponseWriter, r *http.Request) {
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	src, scope, ok := h.scope(w, r)
	if !ok {
		return
	}
	repos, ok := h.enabledRepos(w, r)
	if !ok {
		return
	}
	report, err := metrics.Compute(r.Context(), src, metrics.AllRepos(repos), win, h.cfg.Now(), scope)
	if err != nil {
		h.internalError(w, "compute metrics across repos", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *handlers) getAllMonthlyMetrics(w http.ResponseWriter, r *http.Request) {
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	if err := win.CheckTrend(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	src, scope, ok := h.scope(w, r)
	if !ok {
		return
	}
	repos, ok := h.enabledRepos(w, r)
	if !ok {
		return
	}
	months, err := metrics.ComputeMonthly(r.Context(), src, metrics.AllRepos(repos), win, h.cfg.Now(), scope)
	if err != nil {
		h.internalError(w, "compute monthly metrics across repos", err)
		return
	}
	writeJSON(w, http.StatusOK, monthlyJSON{
		Repo:   metrics.AllReposName,
		From:   win.From.Format("2006-01"),
		To:     win.To.Format("2006-01"),
		Months: months,
	})
}

func (h *handlers) getRepoOverview(w http.ResponseWriter, r *http.Request) {
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	if err := win.CheckTrend(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repos, ok := h.enabledRepos(w, r)
	if !ok {
		return
	}
	ov, err := metrics.ComputeRepoOverview(r.Context(), h.cfg.Metrics, repos, win, h.cfg.Now())
	if err != nil {
		h.internalError(w, "compute repo overview", err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (h *handlers) getMemberOverview(w http.ResponseWriter, r *http.Request) {
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	if err := win.CheckTrend(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repos, ok := h.enabledRepos(w, r)
	if !ok {
		return
	}
	members, err := h.cfg.People.ListMembers(r.Context())
	if err != nil {
		h.internalError(w, "list members", err)
		return
	}
	ov, err := metrics.ComputeMemberOverview(r.Context(), h.cfg.Authors, h.cfg.Scoped, members, repos, win, h.cfg.Now())
	if err != nil {
		h.internalError(w, "compute member overview", err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}
