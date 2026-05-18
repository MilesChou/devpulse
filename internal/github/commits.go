package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// GetCommitAuthorAccountsBulk resolves the GitHub login for each SHA via
// the GraphQL repository.object aliased query.
//
// GitHub's GraphQL does not accept `oid` as a variable, so SHAs are
// concatenated into the query body. Inputs go through commitsha.Parse
// which already enforces 40 lower-case hex chars, eliminating injection
// risk before this code runs.
//
// Batches above graphqlMaxBatch are split automatically. A SHA whose
// author has no associated user.login (e.g. force-pushed commit by an
// account that has since been deleted) maps to nil — distinguishable
// from "missing key" by the caller.
func (c *Client) GetCommitAuthorAccountsBulk(
	ctx context.Context,
	repoName repo.FullName,
	shas []commitsha.SHA,
) (map[commitsha.SHA]*string, error) {
	out := make(map[commitsha.SHA]*string, len(shas))

	for start := 0; start < len(shas); start += graphqlMaxBatch {
		end := start + graphqlMaxBatch
		if end > len(shas) {
			end = len(shas)
		}
		batch := shas[start:end]

		query, aliasIndex := buildBulkAuthorsQuery(batch)
		vars := map[string]any{
			"owner": repoName.Owner,
			"name":  repoName.Name,
		}

		var resp struct {
			Repository map[string]json.RawMessage `json:"repository"`
		}
		if err := c.graphql(ctx, query, vars, &resp); err != nil {
			return nil, err
		}

		for sha, alias := range aliasIndex {
			out[sha] = decodeCommitAuthorLogin(resp.Repository[alias])
		}
	}

	return out, nil
}

// buildBulkAuthorsQuery emits a GraphQL `repository { c0: object... }`
// query for the batch. The returned map points each SHA to its alias.
func buildBulkAuthorsQuery(shas []commitsha.SHA) (string, map[commitsha.SHA]string) {
	aliasIndex := make(map[commitsha.SHA]string, len(shas))

	var body strings.Builder
	for i, sha := range shas {
		alias := fmt.Sprintf("c%d", i)
		aliasIndex[sha] = alias
		body.WriteString(fmt.Sprintf(
			` %s: object(oid: "%s") { ... on Commit { oid author { user { login } } } }`,
			alias, sha,
		))
	}

	return fmt.Sprintf(`query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {%s
  }
}`, body.String()), aliasIndex
}

// decodeCommitAuthorLogin extracts user.login from a commit alias' JSON.
// Returns nil for missing/null author or login.
func decodeCommitAuthorLogin(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	var node struct {
		Author *struct {
			User *struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	if node.Author == nil || node.Author.User == nil || node.Author.User.Login == "" {
		return nil
	}
	login := node.Author.User.Login
	return &login
}

