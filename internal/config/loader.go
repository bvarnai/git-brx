package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
	"gopkg.in/yaml.v3"
)

// FindConfigFile searches for a configuration file across standard hierarchy.
func FindConfigFile(workDir string) string {
	if envPath := os.Getenv("GIT_BRX_CONFIG_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	candidates := []string{
		filepath.Join(workDir, ".git-brx.yaml"),
		filepath.Join(workDir, ".git-brx.yml"),
		filepath.Join(workDir, ".git-brx", "config.yaml"),
		filepath.Join(workDir, ".git-brx", "config.yml"),
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".config", "git-brx", "config.yaml"),
			filepath.Join(home, ".config", "git-brx", "config.yml"),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// Load loads and resolves configuration from files or remote origin auto-discovery.
func Load(workDir, originURL string) (*domain.ProjectConfig, error) {
	configFile := FindConfigFile(workDir)
	cfg := &domain.ProjectConfig{}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, domain.WrapError(domain.ExitConfigError, err, "failed to read configuration file: %s", configFile)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, domain.WrapError(domain.ExitConfigError, err, "failed to parse configuration file: %s", configFile)
		}
	}

	var detected *DetectedRemote
	if originURL != "" {
		detected = DetectRemote(originURL)
	}

	// If platform is not specified, fall back to detected remote platform
	if cfg.Platform == "" {
		if detected != nil {
			cfg.Platform = detected.Platform
		} else if configFile == "" {
			return nil, domain.NewError(domain.ExitConfigError, "No configuration file found and unable to detect platform from remote origin").
				WithHint("Create a .git-brx.yaml configuration file or configure a valid GitHub remote origin")
		}
	}

	// Expand platform presets
	if cfg.Platform != "" && strings.ToLower(cfg.Platform) != "none" {
		expandPlatformPreset(cfg, detected)
	}

	// Backfill missing fields from detected remote where applicable
	if detected != nil && strings.ToLower(cfg.Platform) != "none" {
		if cfg.SCM.Owner == "" && detected.Owner != "" {
			cfg.SCM.Owner = detected.Owner
		}
		if cfg.SCM.Repo == "" && detected.Repo != "" {
			cfg.SCM.Repo = detected.Repo
		}
		if cfg.SCM.URI == "" && detected.BaseURI != "" {
			cfg.SCM.URI = detected.BaseURI
		}
	}

	// Default branch template
	if cfg.Branch.Template == "" {
		projectKey := cfg.Tracker.Project
		if projectKey == "" && cfg.SCM.Project != "" {
			projectKey = cfg.SCM.Project
		}
		if projectKey != "" {
			cfg.Branch.Template = fmt.Sprintf("^(issue|feature|epic)/(%s-[1-9][0-9]*)$", regexpEscape(projectKey))
		} else {
			cfg.Branch.Template = `^(issue|feature|epic)/[A-Za-z0-9_-]+$`
		}
	} else if cfg.Tracker.Project != "" {
		// Interpolate legacy ${projectkey} variable if present
		cfg.Branch.Template = strings.ReplaceAll(cfg.Branch.Template, "${projectkey}", cfg.Tracker.Project)
	}

	// Default branch mapping
	if len(cfg.Branch.Mapping) == 0 {
		cfg.Branch.Mapping = map[string]string{
			"Bug":   "issue",
			"Story": "feature",
			"Epic":  "epic",
			"Task":  "issue",
			"Issue": "issue",
		}
	}

	return cfg, nil
}

func expandPlatformPreset(cfg *domain.ProjectConfig, detected *DetectedRemote) {
	switch strings.ToLower(cfg.Platform) {
	case "github":
		if cfg.Tracker.Provider == "" {
			cfg.Tracker.Provider = "github"
		}
		if cfg.SCM.Provider == "" {
			cfg.SCM.Provider = "github"
		}
		if detected != nil {
			if cfg.Tracker.Owner == "" {
				cfg.Tracker.Owner = detected.Owner
			}
			if cfg.Tracker.Repo == "" {
				cfg.Tracker.Repo = detected.Repo
			}
			if cfg.SCM.Owner == "" {
				cfg.SCM.Owner = detected.Owner
			}
			if cfg.SCM.Repo == "" {
				cfg.SCM.Repo = detected.Repo
			}
		}
	case "gitlab":
		if cfg.Tracker.Provider == "" {
			cfg.Tracker.Provider = "gitlab"
		}
		if cfg.SCM.Provider == "" {
			cfg.SCM.Provider = "gitlab"
		}
		if detected != nil {
			if cfg.Tracker.Owner == "" {
				cfg.Tracker.Owner = detected.Owner
			}
			if cfg.Tracker.Repo == "" {
				cfg.Tracker.Repo = detected.Repo
			}
			if cfg.SCM.Owner == "" {
				cfg.SCM.Owner = detected.Owner
			}
			if cfg.SCM.Repo == "" {
				cfg.SCM.Repo = detected.Repo
			}
		}
	case "bitbucket":
		if cfg.SCM.Provider == "" {
			cfg.SCM.Provider = "bitbucket"
		}
		if cfg.Tracker.Provider == "" {
			cfg.Tracker.Provider = "jira"
		}
	case "none":
		// Explicit opt-out: don't expand any presets
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
