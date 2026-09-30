package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/people"
)

// PeopleStore is the subset of persistence.PeoplePersister the API uses.
type PeopleStore interface {
	ListMembers(ctx context.Context) ([]people.Member, error)
	CreateMember(ctx context.Context, name string, accounts []string) (people.Member, error)
	UpdateMember(ctx context.Context, id, name string, accounts []string) error
	DeleteMember(ctx context.Context, id string) error
	ListTeams(ctx context.Context) ([]people.Team, error)
	CreateTeam(ctx context.Context, name string, memberIDs []string) (people.Team, error)
	UpdateTeam(ctx context.Context, id, name string, memberIDs []string) error
	DeleteTeam(ctx context.Context, id string) error
	ExcludedAccounts(ctx context.Context) ([]string, error)
	ReplaceExcludedAccounts(ctx context.Context, accounts []string) error
	AccountsOfMember(ctx context.Context, id string) (people.Member, error)
	AccountsOfTeam(ctx context.Context, id string) (people.Team, []string, error)
}

func registerPeopleRoutes(api *http.ServeMux, h *handlers) {
	api.HandleFunc("GET /api/v1/members", h.listMembers)
	api.HandleFunc("POST /api/v1/members", h.createMember)
	api.HandleFunc("PUT /api/v1/members/{id}", h.updateMember)
	api.HandleFunc("DELETE /api/v1/members/{id}", h.deleteMember)
	api.HandleFunc("GET /api/v1/teams", h.listTeams)
	api.HandleFunc("POST /api/v1/teams", h.createTeam)
	api.HandleFunc("PUT /api/v1/teams/{id}", h.updateTeam)
	api.HandleFunc("DELETE /api/v1/teams/{id}", h.deleteTeam)
	api.HandleFunc("GET /api/v1/excluded-accounts", h.listExcluded)
	api.HandleFunc("PUT /api/v1/excluded-accounts", h.replaceExcluded)
	api.HandleFunc("GET /api/v1/repos/{owner}/{name}/metrics/by-member", h.getMetricsByMember)
}

type memberJSON struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Accounts    []string `json:"accounts"`
	TeamIDs     []string `json:"team_ids"`
}

func toMemberJSON(m people.Member) memberJSON {
	return memberJSON{ID: m.ID, DisplayName: m.DisplayName, Accounts: nonNilStrings(m.Accounts), TeamIDs: nonNilStrings(m.TeamIDs)}
}

type teamJSON struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

func toTeamJSON(t people.Team) teamJSON {
	return teamJSON{ID: t.ID, Name: t.Name, MemberIDs: nonNilStrings(t.MemberIDs)}
}

type memberBody struct {
	DisplayName string   `json:"display_name"`
	Accounts    []string `json:"accounts"`
}

// normalize validates the body into a display name and accounts.
func (b memberBody) normalize() (string, []string, error) {
	name, err := people.NormalizeName("display_name", b.DisplayName)
	if err != nil {
		return "", nil, err
	}
	accounts, err := people.NormalizeAccounts(b.Accounts)
	return name, accounts, err
}

type teamBody struct {
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

func (h *handlers) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.cfg.People.ListMembers(r.Context())
	if err != nil {
		h.internalError(w, "list members", err)
		return
	}
	out := make([]memberJSON, 0, len(members))
	for _, m := range members {
		out = append(out, toMemberJSON(m))
	}
	writeJSON(w, http.StatusOK, map[string][]memberJSON{"members": out})
}

func (h *handlers) createMember(w http.ResponseWriter, r *http.Request) {
	var body memberBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name, accounts, err := body.normalize()
	if err != nil {
		h.peopleError(w, "create member", err)
		return
	}
	m, err := h.cfg.People.CreateMember(r.Context(), name, accounts)
	if err != nil {
		h.peopleError(w, "create member", err)
		return
	}
	writeJSON(w, http.StatusCreated, toMemberJSON(m))
}

func (h *handlers) updateMember(w http.ResponseWriter, r *http.Request) {
	var body memberBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name, accounts, err := body.normalize()
	if err != nil {
		h.peopleError(w, "update member", err)
		return
	}
	id := r.PathValue("id")
	if err := h.cfg.People.UpdateMember(r.Context(), id, name, accounts); err != nil {
		h.peopleError(w, "update member", err)
		return
	}
	// Re-read through ListMembers, which also loads team memberships, so
	// the answer matches GET /api/v1/members.
	members, err := h.cfg.People.ListMembers(r.Context())
	if err != nil {
		h.internalError(w, "update member", err)
		return
	}
	for _, m := range members {
		if m.ID == id {
			writeJSON(w, http.StatusOK, toMemberJSON(m))
			return
		}
	}
	h.peopleError(w, "update member", people.ErrNotFound)
}

func (h *handlers) deleteMember(w http.ResponseWriter, r *http.Request) {
	if err := h.cfg.People.DeleteMember(r.Context(), r.PathValue("id")); err != nil {
		h.peopleError(w, "delete member", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.cfg.People.ListTeams(r.Context())
	if err != nil {
		h.internalError(w, "list teams", err)
		return
	}
	out := make([]teamJSON, 0, len(teams))
	for _, t := range teams {
		out = append(out, toTeamJSON(t))
	}
	writeJSON(w, http.StatusOK, map[string][]teamJSON{"teams": out})
}

func (h *handlers) createTeam(w http.ResponseWriter, r *http.Request) {
	var body teamBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name, err := people.NormalizeName("name", body.Name)
	if err != nil {
		h.peopleError(w, "create team", err)
		return
	}
	t, err := h.cfg.People.CreateTeam(r.Context(), name, body.MemberIDs)
	if err != nil {
		h.peopleError(w, "create team", err)
		return
	}
	writeJSON(w, http.StatusCreated, toTeamJSON(t))
}

func (h *handlers) updateTeam(w http.ResponseWriter, r *http.Request) {
	var body teamBody
	if !decodeJSON(w, r, &body) {
		return
	}
	name, err := people.NormalizeName("name", body.Name)
	if err != nil {
		h.peopleError(w, "update team", err)
		return
	}
	id := r.PathValue("id")
	if err := h.cfg.People.UpdateTeam(r.Context(), id, name, body.MemberIDs); err != nil {
		h.peopleError(w, "update team", err)
		return
	}
	teams, err := h.cfg.People.ListTeams(r.Context())
	if err != nil {
		h.internalError(w, "update team", err)
		return
	}
	for _, t := range teams {
		if t.ID == id {
			writeJSON(w, http.StatusOK, toTeamJSON(t))
			return
		}
	}
	h.peopleError(w, "update team", people.ErrNotFound)
}

func (h *handlers) deleteTeam(w http.ResponseWriter, r *http.Request) {
	if err := h.cfg.People.DeleteTeam(r.Context(), r.PathValue("id")); err != nil {
		h.peopleError(w, "delete team", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) listExcluded(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.cfg.People.ExcludedAccounts(r.Context())
	if err != nil {
		h.internalError(w, "list excluded accounts", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"accounts": accounts})
}

func (h *handlers) replaceExcluded(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Accounts []string `json:"accounts"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	accounts, err := people.NormalizeAccounts(body.Accounts)
	if err != nil {
		h.peopleError(w, "replace excluded accounts", err)
		return
	}
	if err := h.cfg.People.ReplaceExcludedAccounts(r.Context(), accounts); err != nil {
		h.internalError(w, "replace excluded accounts", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"accounts": accounts})
}

// byMemberJSON answers GET .../metrics/by-member.
type byMemberJSON struct {
	Repo string        `json:"repo"`
	From string        `json:"from"`
	To   string        `json:"to"`
	Rows []metrics.Row `json:"rows"`
}

func (h *handlers) getMetricsByMember(w http.ResponseWriter, r *http.Request) {
	rp, ok := h.findRepo(w, r)
	if !ok {
		return
	}
	win, ok := h.window(w, r)
	if !ok {
		return
	}
	members, err := h.cfg.People.ListMembers(r.Context())
	if err != nil {
		h.internalError(w, "list members", err)
		return
	}
	rows, err := metrics.ComputeByMember(r.Context(), h.cfg.Authors, h.cfg.Scoped, members, metrics.Single(rp), win, h.cfg.Now())
	if err != nil {
		h.internalError(w, "compute metrics by member", err)
		return
	}
	writeJSON(w, http.StatusOK, byMemberJSON{
		Repo: rp.Name.String(),
		From: win.From.Format("2006-01"),
		To:   win.To.Format("2006-01"),
		Rows: rows,
	})
}

// scope resolves the optional ?member= or ?team= query parameter into
// a limited Source and the Scope to report. Without either it returns
// the unscoped Source and a nil Scope. On a bad request it answers and
// returns ok=false.
func (h *handlers) scope(w http.ResponseWriter, r *http.Request) (metrics.Source, *metrics.Scope, bool) {
	q := r.URL.Query()
	memberID, teamID, account := q.Get("member"), q.Get("team"), q.Get("account")
	given := 0
	for _, v := range []string{memberID, teamID, account} {
		if v != "" {
			given++
		}
	}
	switch {
	case given > 1:
		writeError(w, http.StatusBadRequest, "use one of member, team or account, not several")
		return nil, nil, false
	case account != "":
		// An account no member claims yet: the Overview lists such
		// accounts, and they can be looked at before anyone maps them.
		a := people.NormalizeAccount(account)
		if a == "" {
			writeError(w, http.StatusBadRequest, "account must not be blank")
			return nil, nil, false
		}
		accounts := []string{a}
		return h.cfg.Scoped(accounts), &metrics.Scope{Kind: "account", ID: a, Name: a, Accounts: accounts}, true
	case memberID != "":
		m, err := h.cfg.People.AccountsOfMember(r.Context(), memberID)
		if err != nil {
			h.peopleError(w, "find member", err)
			return nil, nil, false
		}
		return h.cfg.Scoped(m.Accounts), &metrics.Scope{Kind: "member", ID: m.ID, Name: m.DisplayName, Accounts: nonNilStrings(m.Accounts)}, true
	case teamID != "":
		t, accounts, err := h.cfg.People.AccountsOfTeam(r.Context(), teamID)
		if err != nil {
			h.peopleError(w, "find team", err)
			return nil, nil, false
		}
		return h.cfg.Scoped(accounts), &metrics.Scope{Kind: "team", ID: t.ID, Name: t.Name, Accounts: nonNilStrings(accounts)}, true
	}
	return h.cfg.Metrics, nil, true
}

// peopleError maps people errors onto statuses: invalid input 400,
// clash with stored data 409, unknown id 404, anything else 500.
func (h *handlers) peopleError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, people.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, people.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, people.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		h.internalError(w, op, err)
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
