package config

import (
	"fmt"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
)

// PlatformPreset defines the configuration conventions and defaults for a specific Git/issue platform.
type PlatformPreset interface {
	Name() string
	Apply(cfg *domain.ProjectConfig, detected *DetectedRemote, inferred map[string]string)
}

// GetPreset returns the PlatformPreset matching platform name, or nil if none matched.
func GetPreset(platform string) PlatformPreset {
	switch strings.ToLower(platform) {
	case "github":
		return &githubPreset{}
	case "gitlab":
		return &gitlabPreset{}
	case "bitbucket":
		return &bitbucketPreset{}
	default:
		return nil
	}
}

// githubPreset provides defaults for GitHub repositories.
type githubPreset struct{}

func (p *githubPreset) Name() string { return "github" }

func (p *githubPreset) Apply(cfg *domain.ProjectConfig, detected *DetectedRemote, inferred map[string]string) {
	if cfg.Tracker.Provider == "" {
		cfg.Tracker.Provider = "github"
		if inferred != nil {
			inferred["tracker.provider"] = "inferred from platform preset"
		}
	}
	if cfg.SCM.Provider == "" {
		cfg.SCM.Provider = "github"
		if inferred != nil {
			inferred["scm.provider"] = "inferred from platform preset"
		}
	}
	if cfg.Branch.Template == "" {
		cfg.Branch.Template = `^(issue|feature|epic)/[1-9][0-9]*$`
		if inferred != nil {
			inferred["branch.template"] = "platform default (github)"
		}
	}
	if len(cfg.Branch.Mapping) == 0 {
		cfg.Branch.Mapping = map[string]any{
			"bug":         "issue",
			"enhancement": []string{"issue", "feature"},
			"feature":     "feature",
			"epic":        "epic",
		}
		if inferred != nil {
			inferred["branch.mapping"] = "platform default (github)"
		}
	}
	if detected != nil {
		if cfg.Tracker.Owner == "" && detected.Owner != "" {
			cfg.Tracker.Owner = detected.Owner
			if inferred != nil {
				inferred["tracker.owner"] = "auto-discovered from remote origin"
			}
		}
		if cfg.Tracker.Repo == "" && detected.Repo != "" {
			cfg.Tracker.Repo = detected.Repo
			if inferred != nil {
				inferred["tracker.repo"] = "auto-discovered from remote origin"
			}
		}
		if cfg.SCM.Owner == "" && detected.Owner != "" {
			cfg.SCM.Owner = detected.Owner
			if inferred != nil {
				inferred["scm.owner"] = "auto-discovered from remote origin"
			}
		}
		if cfg.SCM.Repo == "" && detected.Repo != "" {
			cfg.SCM.Repo = detected.Repo
			if inferred != nil {
				inferred["scm.repo"] = "auto-discovered from remote origin"
			}
		}
	}
}

// gitlabPreset provides defaults for GitLab repositories.
type gitlabPreset struct{}

func (p *gitlabPreset) Name() string { return "gitlab" }

func (p *gitlabPreset) Apply(cfg *domain.ProjectConfig, detected *DetectedRemote, inferred map[string]string) {
	if cfg.Tracker.Provider == "" {
		cfg.Tracker.Provider = "gitlab"
		if inferred != nil {
			inferred["tracker.provider"] = "inferred from platform preset"
		}
	}
	if cfg.SCM.Provider == "" {
		cfg.SCM.Provider = "gitlab"
		if inferred != nil {
			inferred["scm.provider"] = "inferred from platform preset"
		}
	}
	if cfg.Branch.Template == "" {
		cfg.Branch.Template = `^(issue|feature|epic)/[1-9][0-9]*$`
		if inferred != nil {
			inferred["branch.template"] = "platform default (gitlab)"
		}
	}
	if len(cfg.Branch.Mapping) == 0 {
		cfg.Branch.Mapping = map[string]any{
			"bug":         "issue",
			"enhancement": []string{"issue", "feature"},
			"feature":     "feature",
			"epic":        "epic",
		}
		if inferred != nil {
			inferred["branch.mapping"] = "platform default (gitlab)"
		}
	}
	if detected != nil {
		if cfg.Tracker.Owner == "" && detected.Owner != "" {
			cfg.Tracker.Owner = detected.Owner
			if inferred != nil {
				inferred["tracker.owner"] = "auto-discovered from remote origin"
			}
		}
		if cfg.Tracker.Repo == "" && detected.Repo != "" {
			cfg.Tracker.Repo = detected.Repo
			if inferred != nil {
				inferred["tracker.repo"] = "auto-discovered from remote origin"
			}
		}
		if cfg.SCM.Owner == "" && detected.Owner != "" {
			cfg.SCM.Owner = detected.Owner
			if inferred != nil {
				inferred["scm.owner"] = "auto-discovered from remote origin"
			}
		}
		if cfg.SCM.Repo == "" && detected.Repo != "" {
			cfg.SCM.Repo = detected.Repo
			if inferred != nil {
				inferred["scm.repo"] = "auto-discovered from remote origin"
			}
		}
	}
}

// bitbucketPreset provides defaults for Bitbucket and Jira repositories.
type bitbucketPreset struct{}

func (p *bitbucketPreset) Name() string { return "bitbucket" }

func (p *bitbucketPreset) Apply(cfg *domain.ProjectConfig, detected *DetectedRemote, inferred map[string]string) {
	if cfg.SCM.Provider == "" {
		cfg.SCM.Provider = "bitbucket"
		if inferred != nil {
			inferred["scm.provider"] = "inferred from platform preset"
		}
	}
	if cfg.Tracker.Provider == "" {
		cfg.Tracker.Provider = "jira"
		if inferred != nil {
			inferred["tracker.provider"] = "inferred from platform preset"
		}
	}
	if cfg.Branch.Template == "" {
		projectKey := cfg.Tracker.Project
		if projectKey == "" && cfg.SCM.Project != "" {
			projectKey = cfg.SCM.Project
		}
		if projectKey != "" {
			cfg.Branch.Template = fmt.Sprintf("^(issue|feature|epic)/(%s-[1-9][0-9]*)$", regexpEscape(projectKey))
			if inferred != nil {
				inferred["branch.template"] = "platform default (jira)"
			}
		}
	}
	if len(cfg.Branch.Mapping) == 0 {
		cfg.Branch.Mapping = map[string]any{
			"Bug":   "issue",
			"Story": "feature",
			"Epic":  "epic",
			"Task":  "issue",
		}
		if inferred != nil {
			inferred["branch.mapping"] = "platform default (jira)"
		}
	}
}

func regexpEscape(s string) string {
	special := `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
