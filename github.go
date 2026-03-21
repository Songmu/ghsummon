package ghsummon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type ghClient struct {
	client     *github.Client
	httpClient *http.Client
	owner      string
	repo       string
}

func newGHClient(ctx context.Context, token, ownerRepo string) (*ghClient, error) {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GITHUB_REPOSITORY format: %q (expected owner/repo)", ownerRepo)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	rateLimitClient := github_ratelimit.NewClient(httpClient.Transport)
	client := github.NewClient(rateLimitClient)
	return &ghClient{
		client:     client,
		httpClient: rateLimitClient,
		owner:      parts[0],
		repo:       parts[1],
	}, nil
}

// getDefaultBranch returns the repository's default branch name.
func (g *ghClient) getDefaultBranch(ctx context.Context) (string, error) {
	repo, _, err := g.client.Repositories.Get(ctx, g.owner, g.repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}
	return repo.GetDefaultBranch(), nil
}

func (g *ghClient) branchExists(ctx context.Context, branch string) (bool, error) {
	ref := "refs/heads/" + branch
	_, resp, err := g.client.Git.GetRef(ctx, g.owner, g.repo, ref)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check branch %s: %w", branch, err)
	}
	return true, nil
}

// createEmptyCommitAndBranch creates an empty commit on a new branch via Git Data API.
func (g *ghClient) createEmptyCommitAndBranch(ctx context.Context, baseBranch, branch, message string) error {
	// Get default branch HEAD ref
	defaultRef, _, err := g.client.Git.GetRef(ctx, g.owner, g.repo, "refs/heads/"+baseBranch)
	if err != nil {
		return fmt.Errorf("failed to get HEAD ref: %w", err)
	}
	headSHA := defaultRef.Object.GetSHA()

	// Get the commit to obtain the tree SHA
	commit, _, err := g.client.Git.GetCommit(ctx, g.owner, g.repo, headSHA)
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}
	treeSHA := commit.Tree.GetSHA()

	// Create an empty commit (same tree = no file changes)
	newCommit, _, err := g.client.Git.CreateCommit(ctx, g.owner, g.repo,
		&github.Commit{
			Message: github.Ptr(message),
			Tree:    &github.Tree{SHA: github.Ptr(treeSHA)},
			Parents: []*github.Commit{{SHA: github.Ptr(headSHA)}},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Create the branch
	ref := "refs/heads/" + branch
	_, _, err = g.client.Git.CreateRef(ctx, g.owner, g.repo, &github.Reference{
		Ref:    github.Ptr(ref),
		Object: &github.GitObject{SHA: newCommit.SHA},
	})
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branch, err)
	}

	return nil
}

// createPR creates a pull request and returns its number and node ID.
func (g *ghClient) createPR(ctx context.Context, baseBranch, branch, title, body string) (int, string, error) {
	pr, _, err := g.client.PullRequests.Create(ctx, g.owner, g.repo, &github.NewPullRequest{
		Title: github.Ptr(title),
		Head:  github.Ptr(branch),
		Base:  github.Ptr(baseBranch),
		Body:  github.Ptr(body),
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to create PR: %w", err)
	}
	return pr.GetNumber(), pr.GetNodeID(), nil
}

// assignCopilot assigns the Copilot Coding Agent to a PR via GraphQL API.
// The REST API silently ignores bot assignees, so GraphQL is required.
func (g *ghClient) assignCopilot(ctx context.Context, prNodeID string) error {
	agentID, err := g.getCopilotAgentID(ctx)
	if err != nil {
		return err
	}

	mutation := `mutation($assignableId: ID!, $assigneeIds: [ID!]!) {
		addAssigneesToAssignable(input: {assignableId: $assignableId, assigneeIds: $assigneeIds}) {
			assignable { __typename }
		}
	}`
	variables := map[string]any{
		"assignableId": prNodeID,
		"assigneeIds":  []string{agentID},
	}
	var result struct {
		Data struct {
			AddAssigneesToAssignable struct {
				Assignable struct {
					TypeName string `json:"__typename"`
				} `json:"assignable"`
			} `json:"addAssigneesToAssignable"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.graphql(ctx, mutation, variables, &result); err != nil {
		return fmt.Errorf("failed to assign copilot: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("GraphQL error assigning copilot: %s", result.Errors[0].Message)
	}
	return nil
}

// getCopilotAgentID finds the Copilot agent's node ID via suggestedActors GraphQL query.
func (g *ghClient) getCopilotAgentID(ctx context.Context) (string, error) {
	query := `query($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			suggestedActors(capabilities: [CAN_BE_ASSIGNED], first: 100) {
				nodes {
					login
					__typename
					... on Bot { id }
				}
			}
		}
	}`
	variables := map[string]any{
		"owner": g.owner,
		"name":  g.repo,
	}
	var result struct {
		Data struct {
			Repository struct {
				SuggestedActors struct {
					Nodes []struct {
						Login    string `json:"login"`
						TypeName string `json:"__typename"`
						ID       string `json:"id"`
					} `json:"nodes"`
				} `json:"suggestedActors"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.graphql(ctx, query, variables, &result); err != nil {
		return "", fmt.Errorf("failed to query suggested actors: %w", err)
	}
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("GraphQL error querying actors: %s", result.Errors[0].Message)
	}

	for _, node := range result.Data.Repository.SuggestedActors.Nodes {
		if node.Login == "copilot-swe-agent" || node.Login == "copilot" {
			if node.ID != "" {
				return node.ID, nil
			}
		}
	}
	return "", fmt.Errorf("copilot agent not found in suggested actors; ensure Copilot Coding Agent is enabled for this repository")
}

// graphql executes a GraphQL query/mutation against the GitHub API.
func (g *ghClient) graphql(ctx context.Context, query string, variables map[string]any, result any) error {
	body := map[string]any{
		"query":     query,
		"variables": variables,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	return json.Unmarshal(respBody, result)
}

// postCopilotComment posts an @copilot comment on the PR to trigger the Coding Agent.
func (g *ghClient) postCopilotComment(ctx context.Context, prNumber int, comment string) error {
	_, _, err := g.client.Issues.CreateComment(ctx, g.owner, g.repo, prNumber, &github.IssueComment{
		Body: github.Ptr(comment),
	})
	if err != nil {
		return fmt.Errorf("failed to post comment on PR #%d: %w", prNumber, err)
	}
	return nil
}

// buildPRBody generates the PR body for a given prompt.
func buildPRBody(p Prompt) string {
	return fmt.Sprintf(`This PR was automatically created by [ghsummon](https://github.com/Songmu/ghsummon).

**Target file**: %s

**Prompt**:
> %s
`, "`"+p.FilePath+"`", strings.ReplaceAll(p.Text, "\n", "\n> "))
}

// buildCopilotComment generates the @copilot comment for a given prompt.
func buildCopilotComment(p Prompt) string {
	return fmt.Sprintf(`@copilot

Please replace the %s prompt line(s) in the following file
with your research results.

**Target file**: %s

**Prompt**:
> %s

**Instructions**:
- Remove the %s prompt line(s) (including continuation lines) and replace them with your research results
- Write the results in Markdown format
- Cite sources (URLs, etc.) for factual claims
- Preserve the existing file structure (heading levels, etc.)
- Search the web for up-to-date information when needed
`, "`@copilot`", "`"+p.FilePath+"`",
		strings.ReplaceAll(p.Text, "\n", "\n> "),
		"`@copilot`")
}

// buildMultiPRBody generates the PR body when a file has multiple prompts.
func buildMultiPRBody(filePath string, prompts []Prompt) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "This PR was automatically created by [ghsummon](https://github.com/Songmu/ghsummon).\n\n")
	fmt.Fprintf(&sb, "**Target file**: `%s`\n\n", filePath)
	for i, p := range prompts {
		fmt.Fprintf(&sb, "**Prompt %d** (line %d):\n> %s\n\n",
			i+1, p.StartLine, strings.ReplaceAll(p.Text, "\n", "\n> "))
	}
	return sb.String()
}

// buildMultiCopilotComment generates the @copilot comment when a file has multiple prompts.
func buildMultiCopilotComment(filePath string, prompts []Prompt) string {
	var sb strings.Builder
	sb.WriteString("@copilot\n\n")
	fmt.Fprintf(&sb, "Please replace all `@copilot` prompt line(s) in the following file\nwith your research results.\n\n")
	fmt.Fprintf(&sb, "**Target file**: `%s`\n\n", filePath)
	for i, p := range prompts {
		fmt.Fprintf(&sb, "**Prompt %d** (line %d):\n> %s\n\n",
			i+1, p.StartLine, strings.ReplaceAll(p.Text, "\n", "\n> "))
	}
	sb.WriteString("**Instructions**:\n")
	sb.WriteString("- Remove each `@copilot` prompt line(s) (including continuation lines) and replace them with your research results\n")
	sb.WriteString("- Write the results in Markdown format\n")
	sb.WriteString("- Cite sources (URLs, etc.) for factual claims\n")
	sb.WriteString("- Preserve the existing file structure (heading levels, etc.)\n")
	sb.WriteString("- Search the web for up-to-date information when needed\n")
	return sb.String()
}
