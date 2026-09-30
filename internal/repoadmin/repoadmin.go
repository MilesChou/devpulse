// Package repoadmin holds the operations that change which repos are
// tracked and how: registering, removing, and per-repo settings. The CLI
// (`devpulse repo ...`) and the HTTP API both call it, so validation
// rules and the "register, then best-effort fetch metadata" flow exist
// once.
package repoadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

// Provider is the VCS host every repo registered here belongs to.
const Provider = "github"

// Store is the subset of persistence.RepoPersister the service needs.
type Store interface {
	FindByFullName(ctx context.Context, provider string, fullName repo.FullName) (repo.Repo, error)
	EnsureID(ctx context.Context, provider string, fullName repo.FullName) (repo.Repo, error)
	UpdateMetadata(ctx context.Context, id string, meta repo.Repo) error
	UpdatePRSyncStart(ctx context.Context, id string, n int) error
	UpdateLabels(ctx context.Context, id, incidentLabel, hotfixLabel string) error
	Delete(ctx context.Context, id string) error
}

var _ Store = (*persistence.RepoPersister)(nil)

// MetadataFetcher fetches a repo's GitHub metadata (description, default
// branch, disabled). fetching.VCSProvider satisfies it.
type MetadataFetcher interface {
	GetRepo(ctx context.Context, name repo.FullName) (repo.Repo, error)
}

// ErrInvalidConfig classifies a rejected setting value, so callers can
// report a usage error (CLI) or a 400 (API) rather than a failure.
var ErrInvalidConfig = errors.New("repoadmin: invalid setting")

// ErrNotFound is returned for an operation on a repo that is not
// tracked. It wraps persistence.ErrRepoNotFound.
var ErrNotFound = persistence.ErrRepoNotFound

// Service implements the operations.
type Service struct {
	store Store
	meta  MetadataFetcher
}

func New(store Store, meta MetadataFetcher) *Service {
	return &Service{store: store, meta: meta}
}

// Registration is the outcome of Register.
type Registration struct {
	Repo    repo.Repo
	Created bool // false when the repo was already tracked
	// MetadataErr is set when GitHub metadata could not be fetched or
	// stored. The repo is still registered, as with `devpulse repo add`:
	// a typo'd or private repo shows up with empty metadata and the
	// next `devpulse repo refresh` can fill it in.
	MetadataErr error
}

// Register tracks the repo (idempotently) and best-effort refreshes its
// GitHub metadata.
func (s *Service) Register(ctx context.Context, name repo.FullName) (Registration, error) {
	var reg Registration

	_, err := s.store.FindByFullName(ctx, Provider, name)
	switch {
	case errors.Is(err, persistence.ErrRepoNotFound):
		reg.Created = true
	case err != nil:
		return reg, fmt.Errorf("find repo %s: %w", name, err)
	}

	r, err := s.store.EnsureID(ctx, Provider, name)
	if err != nil {
		return reg, fmt.Errorf("ensure repo %s: %w", name, err)
	}
	reg.Repo = r

	meta, err := s.meta.GetRepo(ctx, name)
	if err != nil {
		reg.MetadataErr = fmt.Errorf("fetch github metadata: %w", err)
		return reg, nil
	}
	if err := s.store.UpdateMetadata(ctx, r.ID, meta); err != nil {
		reg.MetadataErr = fmt.Errorf("update metadata: %w", err)
		return reg, nil
	}
	reg.Repo.Description = meta.Description
	reg.Repo.DefaultBranch = meta.DefaultBranch
	reg.Repo.Disabled = meta.Disabled
	return reg, nil
}

// Find returns the tracked repo, or ErrNotFound.
func (s *Service) Find(ctx context.Context, name repo.FullName) (repo.Repo, error) {
	return s.store.FindByFullName(ctx, Provider, name)
}

// Remove stops tracking the repo and deletes everything synced for it.
func (s *Service) Remove(ctx context.Context, name repo.FullName) error {
	r, err := s.Find(ctx, name)
	if err != nil {
		return err
	}
	return s.store.Delete(ctx, r.ID)
}

// Config is a partial update of the operator settings: nil fields keep
// their current value.
type Config struct {
	PRStart       *int
	IncidentLabel *string
	HotfixLabel   *string
}

// ValidatePRStart checks the PR-sync floor.
func ValidatePRStart(n int) error {
	if n < 1 {
		return fmt.Errorf("%w: pr-start must be >= 1, got %d", ErrInvalidConfig, n)
	}
	return nil
}

// NormalizeLabel trims a label and rejects a blank one: an empty label
// would match nothing and silently zero out the DORA metric it feeds.
func NormalizeLabel(key, label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", fmt.Errorf("%w: %s must not be blank", ErrInvalidConfig, key)
	}
	return label, nil
}

// UpdateConfig validates every provided field before writing any, so a
// request with one bad field changes nothing, then returns the updated
// repo.
func (s *Service) UpdateConfig(ctx context.Context, r repo.Repo, c Config) (repo.Repo, error) {
	if c.PRStart != nil {
		if err := ValidatePRStart(*c.PRStart); err != nil {
			return r, err
		}
	}
	incident, hotfix := r.IncidentLabel, r.HotfixLabel
	var err error
	if c.IncidentLabel != nil {
		if incident, err = NormalizeLabel("incident-label", *c.IncidentLabel); err != nil {
			return r, err
		}
	}
	if c.HotfixLabel != nil {
		if hotfix, err = NormalizeLabel("hotfix-label", *c.HotfixLabel); err != nil {
			return r, err
		}
	}

	if c.PRStart != nil {
		if err := s.store.UpdatePRSyncStart(ctx, r.ID, *c.PRStart); err != nil {
			return r, err
		}
		r.PRSyncStartNumber = *c.PRStart
	}
	if c.IncidentLabel != nil || c.HotfixLabel != nil {
		if err := s.store.UpdateLabels(ctx, r.ID, incident, hotfix); err != nil {
			return r, err
		}
		r.IncidentLabel, r.HotfixLabel = incident, hotfix
	}
	return r, nil
}
