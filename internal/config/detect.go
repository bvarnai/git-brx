package config

import (
	"net/url"
	"regexp"
	"strings"
)

// DetectedRemote contains components extracted from a Git remote URL.
type DetectedRemote struct {
	Platform string // "github", "gitlab", "bitbucket", or ""
	Host     string
	Owner    string // Owner or Project Key
	Repo     string // Repository slug
	BaseURI  string
}

// sshRegex matches SCP-like git URLs: git@host:owner/repo.git
var scpRegex = regexp.MustCompile(`^(?:([^@]+)@)?([^:]+):/?(.+?)(?:\.git)?$`)

// DetectRemote parses a git remote URL into structured endpoint details.
func DetectRemote(rawURL string) *DetectedRemote {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}

	var host, path string

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "ssh://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return nil
		}
		host = u.Hostname()
		path = strings.TrimPrefix(u.Path, "/")
	} else if matches := scpRegex.FindStringSubmatch(trimmed); len(matches) == 4 {
		host = matches[2]
		path = matches[3]
	} else {
		return nil
	}

	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimPrefix(path, "/")

	// Strip Bitbucket Server /scm/ prefix if present: /scm/PROJ/repo -> PROJ/repo
	path = strings.TrimPrefix(path, "scm/")

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil
	}

	owner := parts[0]
	repo := parts[len(parts)-1]

	// Normalize GitLab nested subgroups if any (owner is top group or full group path)
	if len(parts) > 2 && strings.Contains(host, "gitlab") {
		owner = strings.Join(parts[:len(parts)-1], "/")
	}

	platform := ""
	lowerHost := strings.ToLower(host)
	switch {
	case strings.Contains(lowerHost, "github"):
		platform = "github"
	case strings.Contains(lowerHost, "gitlab"):
		platform = "gitlab"
	case strings.Contains(lowerHost, "bitbucket"):
		platform = "bitbucket"
	}

	scheme := "https"
	if strings.HasPrefix(trimmed, "http://") {
		scheme = "http"
	}

	baseURI := scheme + "://" + host

	return &DetectedRemote{
		Platform: platform,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
		BaseURI:  baseURI,
	}
}
