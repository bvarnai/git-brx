package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
)

// Client interacts with the GitHub REST API and implements IssueTracker and SCMProvider.
type Client struct {
	BaseURL    string // e.g. "https://api.github.com"
	Owner      string
	Repo       string
	Token      string
	ReviewCfg  *domain.ReviewConfig
	HTTPClient *http.Client
}

// Option allows configuring the GitHub client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client (e.g. for testing).
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

// WithBaseURL overrides the default GitHub API base URL (for GitHub Enterprise or testing).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		trimmed := strings.TrimSuffix(baseURL, "/")
		if trimmed == "https://github.com" || trimmed == "http://github.com" {
			c.BaseURL = "https://api.github.com"
		} else {
			c.BaseURL = trimmed
		}
	}
}

// WithReviewConfig passes reviewer routing configurations.
func WithReviewConfig(reviewCfg *domain.ReviewConfig) Option {
	return func(c *Client) {
		c.ReviewCfg = reviewCfg
	}
}

// NewClient creates a new GitHub Client.
func NewClient(owner, repo, token string, opts ...Option) *Client {
	c := &Client{
		BaseURL: "https://api.github.com",
		Owner:   owner,
		Repo:    repo,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Name identifies this provider.
func (c *Client) Name() string {
	return "github"
}

// --- domain.IssueTracker Implementation ---

type rawGitHubIssue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignee *struct {
		Login string `json:"login"`
	} `json:"assignee"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
}

type gitHubErrorResponse struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
	Errors           []struct {
		Resource string `json:"resource"`
		Field    string `json:"field"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"errors"`
}

// GetIssue fetches an issue by its number or key.
func (c *Client) GetIssue(ctx context.Context, key string) (*domain.Issue, error) {
	cleanKey := strings.TrimPrefix(key, "#")
	cleanKey = strings.TrimPrefix(cleanKey, "issue/")
	cleanKey = strings.TrimPrefix(cleanKey, "feature/")

	issueNum, err := strconv.Atoi(cleanKey)
	if err != nil {
		return nil, domain.NewError(domain.ExitUsageError, "Invalid GitHub issue number: %s", key)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", c.BaseURL, c.Owner, c.Repo, issueNum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.WrapError(domain.ExitGeneralError, err, "failed to create HTTP request")
	}

	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, domain.WrapError(domain.ExitPreconditionRemote, err, "GitHub API request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, "fetch issue #%d", issueNum)
	}

	var raw rawGitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, domain.WrapError(domain.ExitAPIError, err, "failed to parse GitHub issue response")
	}

	issueType := "Issue"
	var components []string
	for _, l := range raw.Labels {
		name := l.Name
		components = append(components, name)
		lower := strings.ToLower(name)
		if lower == "bug" || lower == "defect" {
			issueType = "Bug"
		} else if lower == "enhancement" || lower == "feature" || lower == "story" {
			issueType = "Story"
		} else if lower == "epic" {
			issueType = "Epic"
		}
	}

	assignee := ""
	if raw.Assignee != nil && raw.Assignee.Login != "" {
		assignee = raw.Assignee.Login
	} else if len(raw.Assignees) > 0 {
		assignee = raw.Assignees[0].Login
	}

	return &domain.Issue{
		Key:        strconv.Itoa(raw.Number),
		Title:      raw.Title,
		Type:       issueType,
		Status:     raw.State,
		Assignee:   assignee,
		Components: components,
		URL:        raw.HTMLURL,
	}, nil
}

// --- domain.SCMProvider Implementation ---

type rawCreatePRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

type rawPRResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

type rawReviewersRequest struct {
	Reviewers []string `json:"reviewers"`
}

// CreatePullRequest opens a pull request and optionally assigns reviewers.
func (c *Client) CreatePullRequest(ctx context.Context, req domain.PullRequestRequest) (*domain.PullRequestResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.BaseURL, c.Owner, c.Repo)

	payload := rawCreatePRRequest{
		Title: req.Title,
		Body:  req.Description,
		Head:  req.SourceBranch,
		Base:  req.TargetBranch,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, domain.WrapError(domain.ExitGeneralError, err, "failed to serialize pull request payload")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, domain.WrapError(domain.ExitGeneralError, err, "failed to create HTTP request")
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, domain.WrapError(domain.ExitPreconditionRemote, err, "GitHub pull request creation failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.handleErrorResponse(resp, "create pull request for %s", req.SourceBranch)
	}

	var rawPR rawPRResponse
	if err := json.NewDecoder(resp.Body).Decode(&rawPR); err != nil {
		return nil, domain.WrapError(domain.ExitAPIError, err, "failed to parse pull request response")
	}

	// Request reviewers if specified
	if len(req.Reviewers) > 0 {
		c.requestReviewers(ctx, rawPR.Number, req.Reviewers)
	}

	return &domain.PullRequestResult{
		ID:  strconv.Itoa(rawPR.Number),
		URL: rawPR.HTMLURL,
	}, nil
}

type rawPRQueryItem struct {
	Number   int     `json:"number"`
	HTMLURL  string  `json:"html_url"`
	State    string  `json:"state"`
	MergedAt *string `json:"merged_at"`
}

// GetPullRequestForBranch queries GitHub for an existing pull request associated with the given branch.
func (c *Client) GetPullRequestForBranch(ctx context.Context, branch string) (*domain.PullRequestDetail, error) {
	// Query: GET /repos/{owner}/{repo}/pulls?head={owner}:{branch}&state=all
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s:%s&state=all", c.BaseURL, c.Owner, c.Repo, c.Owner, branch)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.WrapError(domain.ExitGeneralError, err, "failed to create HTTP request")
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, domain.WrapError(domain.ExitPreconditionRemote, err, "GitHub API request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, "query pull request for branch %s", branch)
	}

	var rawList []rawPRQueryItem
	if err := json.NewDecoder(resp.Body).Decode(&rawList); err != nil {
		return nil, domain.WrapError(domain.ExitAPIError, err, "failed to parse pull request list response")
	}

	if len(rawList) == 0 {
		return nil, nil // No PR found for branch
	}

	pr := rawList[0]
	state := domain.PRStateOpen
	merged := pr.MergedAt != nil && *pr.MergedAt != ""

	if merged {
		state = domain.PRStateMerged
	} else if strings.EqualFold(pr.State, "closed") {
		state = domain.PRStateClosed
	}

	return &domain.PullRequestDetail{
		ID:     strconv.Itoa(pr.Number),
		Number: pr.Number,
		URL:    pr.HTMLURL,
		State:  state,
		Merged: merged,
	}, nil
}

// ResolveReviewers maps issue labels or components to reviewers.
func (c *Client) ResolveReviewers(components []string) []string {
	return config.ResolveReviewers(c.ReviewCfg, components)
}

// FormatMergeInstructions renders standard GitHub-aligned merge instructions.
func (c *Client) FormatMergeInstructions(opts domain.MergeInstructionOptions) string {
	return domain.FormatDefaultMergeInstructions(opts)
}

func (c *Client) requestReviewers(ctx context.Context, prNumber int, reviewers []string) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/requested_reviewers", c.BaseURL, c.Owner, c.Repo, prNumber)
	payload := rawReviewersRequest{Reviewers: reviewers}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}

	c.setHeaders(httpReq)
	resp, err := c.HTTPClient.Do(httpReq)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

func (c *Client) handleErrorResponse(resp *http.Response, actionFormat string, args ...any) error {
	action := fmt.Sprintf(actionFormat, args...)
	body, _ := io.ReadAll(resp.Body)

	var ghErr gitHubErrorResponse
	_ = json.Unmarshal(body, &ghErr)
	msg := strings.TrimSpace(ghErr.Message)
	if len(ghErr.Errors) > 0 {
		var errDetails []string
		for _, e := range ghErr.Errors {
			if e.Message != "" {
				errDetails = append(errDetails, e.Message)
			} else if e.Field != "" && e.Code != "" {
				errDetails = append(errDetails, fmt.Sprintf("%s: %s", e.Field, e.Code))
			}
		}
		if len(errDetails) > 0 {
			if msg != "" {
				msg = fmt.Sprintf("%s (%s)", msg, strings.Join(errDetails, "; "))
			} else {
				msg = strings.Join(errDetails, "; ")
			}
		}
	}
	if msg == "" {
		bodyStr := strings.TrimSpace(string(body))
		// If the response is HTML, don't dump raw HTML markup into terminal
		if strings.HasPrefix(strings.ToLower(bodyStr), "<!doctype") || strings.HasPrefix(strings.ToLower(bodyStr), "<html") {
			msg = fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		} else {
			msg = bodyStr
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.NewError(domain.ExitAPIError, "GitHub authorization failed (%d): %s", resp.StatusCode, msg).
			WithHint("Ensure GITHUB_TOKEN or GH_TOKEN is set in your environment with 'pull_requests: write' scope")
	case http.StatusNotFound:
		err := domain.NewError(domain.ExitAPIError, "GitHub resource not found during %s: %s", action, msg)
		if c.Token == "" {
			err.WithHint("If the repository or issue is private, set GITHUB_TOKEN or GH_TOKEN, or use --offline")
		} else {
			err.WithHint("Verify the issue exists on GitHub or check repository permissions")
		}
		return err
	case http.StatusUnprocessableEntity:
		return domain.NewError(domain.ExitAPIError, "GitHub validation failed during %s (%d): %s", action, resp.StatusCode, msg)
	default:
		return domain.NewError(domain.ExitAPIError, "GitHub API returned status %d during %s: %s", resp.StatusCode, action, msg)
	}
}
