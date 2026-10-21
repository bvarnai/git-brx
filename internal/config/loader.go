package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
	"gopkg.in/yaml.v3"
)

// FindUserConfigFile searches for user-level global configuration in $HOME.
func FindUserConfigFile() string {
	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
	}

	candidates := []string{
		filepath.Join(home, ".config", "git-brx", "config.yaml"),
		filepath.Join(home, ".config", "git-brx", "config.yml"),
		filepath.Join(home, ".git-brx.yaml"),
		filepath.Join(home, ".git-brx.yml"),
		filepath.Join(home, ".git-brx", "config.yaml"),
		filepath.Join(home, ".git-brx", "config.yml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// FindRepoConfigFile searches for repository-level configuration in workDir.
func FindRepoConfigFile(workDir string) string {
	if workDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(workDir, ".git-brx.yaml"),
		filepath.Join(workDir, ".git-brx.yml"),
		filepath.Join(workDir, ".git-brx", "config.yaml"),
		filepath.Join(workDir, ".git-brx", "config.yml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func loadConfigFile(path string) (*domain.ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, domain.WrapError(domain.ExitConfigError, err, "failed to read configuration file: %s", path)
	}

	cfg := &domain.ProjectConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, domain.WrapError(domain.ExitConfigError, err, "failed to parse configuration file: %s", path)
	}
	return cfg, nil
}

// Inspect evaluates and resolves configuration across all 4 tiers, tracking source paths and auto-discovery provenance.
func Inspect(workDir, originURL string) (*ConfigReport, error) {
	report := &ConfigReport{
		InferredFields: make(map[string]string),
	}

	var cfg *domain.ProjectConfig
	foundAnyConfig := false

	// Tier 1: User Global
	if userPath := FindUserConfigFile(); userPath != "" {
		report.UserConfigPath = userPath
		userCfg, err := loadConfigFile(userPath)
		if err != nil {
			return nil, err
		}
		cfg = userCfg
		foundAnyConfig = true
	}

	// Tier 2: Shared Global (GIT_BRX_CONFIG)
	if envPath := os.Getenv("GIT_BRX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return nil, domain.NewError(domain.ExitConfigError, "configuration file specified by GIT_BRX_CONFIG not found: %s", envPath)
		}
		report.SharedConfigPath = envPath
		envCfg, err := loadConfigFile(envPath)
		if err != nil {
			return nil, err
		}
		cfg = MergeConfigs(cfg, envCfg)
		foundAnyConfig = true
	}

	// Tier 3: Repository Local
	if repoPath := FindRepoConfigFile(workDir); repoPath != "" {
		report.RepoConfigPath = repoPath
		repoCfg, err := loadConfigFile(repoPath)
		if err != nil {
			return nil, err
		}
		cfg = MergeConfigs(cfg, repoCfg)
		foundAnyConfig = true
	}

	if cfg == nil {
		cfg = &domain.ProjectConfig{}
	}

	report.OriginURL = originURL
	var detected *DetectedRemote
	if originURL != "" {
		detected = DetectRemote(originURL)
	}

	// If platform is not specified, fall back to detected remote platform
	if cfg.Platform == "" {
		if detected != nil {
			cfg.Platform = detected.Platform
			report.InferredFields["platform"] = "auto-discovered from remote origin"
		} else if !foundAnyConfig {
			return nil, domain.NewError(domain.ExitConfigError, "No configuration file found and unable to detect platform from remote origin").
				WithHint("Create a .git-brx.yaml configuration file or configure a valid GitHub remote origin")
		}
	}

	// Expand platform presets
	if cfg.Platform != "" && strings.ToLower(cfg.Platform) != "none" {
		if preset := GetPreset(cfg.Platform); preset != nil {
			preset.Apply(cfg, detected, report.InferredFields)
		}
	}

	// Backfill missing fields from detected remote where applicable
	if detected != nil && strings.ToLower(cfg.Platform) != "none" {
		if cfg.SCM.Owner == "" && detected.Owner != "" {
			cfg.SCM.Owner = detected.Owner
			report.InferredFields["scm.owner"] = "auto-discovered from remote origin"
		}
		if cfg.SCM.Repo == "" && detected.Repo != "" {
			cfg.SCM.Repo = detected.Repo
			report.InferredFields["scm.repo"] = "auto-discovered from remote origin"
		}
		if cfg.SCM.URI == "" && detected.BaseURI != "" {
			cfg.SCM.URI = detected.BaseURI
			report.InferredFields["scm.uri"] = "auto-discovered from remote origin"
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
			report.InferredFields["branch.template"] = "inferred from project key"
		} else {
			cfg.Branch.Template = `^(issue|feature|epic)/[A-Za-z0-9_-]+$`
			report.InferredFields["branch.template"] = "default convention"
		}
	}

	// Default branch mapping
	if len(cfg.Branch.Mapping) == 0 {
		cfg.Branch.Mapping = map[string]any{
			"Bug":         "issue",
			"Story":       "feature",
			"Epic":        "epic",
			"Task":        "issue",
			"Issue":       "issue",
			"enhancement": []string{"issue", "feature"},
		}
		report.InferredFields["branch.mapping"] = "default convention"
	}

	report.Config = cfg
	return report, nil
}

// Load loads and resolves configuration across a 4-tier cascade:
// Tier 1: User / Machine Global (~/.config/git-brx/config.yaml, ~/.git-brx.yaml)
// Tier 2: Shared Global (GIT_BRX_CONFIG)
// Tier 3: Repository Local (.git-brx.yaml in workDir)
// Tier 4: Remote Origin Auto-Discovery (git remote origin)
func Load(workDir, originURL string) (*domain.ProjectConfig, error) {
	report, err := Inspect(workDir, originURL)
	if err != nil {
		return nil, err
	}
	return report.Config, nil
}
