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
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
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

// FindConfigFile returns the active configuration file path for backward compatibility.
func FindConfigFile(workDir string) string {
	if envPath := os.Getenv("GIT_BRX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}
	if repoPath := FindRepoConfigFile(workDir); repoPath != "" {
		return repoPath
	}
	return FindUserConfigFile()
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

// Load loads and resolves configuration across a 4-tier cascade:
// Tier 1: User / Machine Global (~/.config/git-brx/config.yaml, ~/.git-brx.yaml)
// Tier 2: Company / Shared Global (GIT_BRX_CONFIG)
// Tier 3: Repository Local (.git-brx.yaml in workDir)
// Tier 4: Remote Origin Auto-Discovery (git remote origin)
func Load(workDir, originURL string) (*domain.ProjectConfig, error) {
	var cfg *domain.ProjectConfig
	foundAnyConfig := false

	// Tier 1: User Global
	if userPath := FindUserConfigFile(); userPath != "" {
		userCfg, err := loadConfigFile(userPath)
		if err != nil {
			return nil, err
		}
		cfg = userCfg
		foundAnyConfig = true
	}

	// Tier 2: Company / Shared Global (GIT_BRX_CONFIG)
	if envPath := os.Getenv("GIT_BRX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return nil, domain.NewError(domain.ExitConfigError, "configuration file specified by GIT_BRX_CONFIG not found: %s", envPath)
		}
		envCfg, err := loadConfigFile(envPath)
		if err != nil {
			return nil, err
		}
		cfg = MergeConfigs(cfg, envCfg)
		foundAnyConfig = true
	}

	// Tier 3: Repository Local
	if repoPath := FindRepoConfigFile(workDir); repoPath != "" {
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

	var detected *DetectedRemote
	if originURL != "" {
		detected = DetectRemote(originURL)
	}

	// If platform is not specified, fall back to detected remote platform
	if cfg.Platform == "" {
		if detected != nil {
			cfg.Platform = detected.Platform
		} else if !foundAnyConfig {
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

// MergeConfigs overlays override on top of base and returns a new merged ProjectConfig.
func MergeConfigs(base, override *domain.ProjectConfig) *domain.ProjectConfig {
	if base == nil && override == nil {
		return &domain.ProjectConfig{}
	}
	if base == nil {
		return cloneConfig(override)
	}
	if override == nil {
		return cloneConfig(base)
	}

	res := cloneConfig(base)

	// Platform
	if override.Platform != "" {
		res.Platform = override.Platform
	}

	// Tracker
	if override.Tracker.Provider != "" {
		res.Tracker.Provider = override.Tracker.Provider
	}
	if override.Tracker.URI != "" {
		res.Tracker.URI = override.Tracker.URI
	}
	if override.Tracker.Project != "" {
		res.Tracker.Project = override.Tracker.Project
	}
	if override.Tracker.Owner != "" {
		res.Tracker.Owner = override.Tracker.Owner
	}
	if override.Tracker.Repo != "" {
		res.Tracker.Repo = override.Tracker.Repo
	}

	// SCM
	if override.SCM.Provider != "" {
		res.SCM.Provider = override.SCM.Provider
	}
	if override.SCM.URI != "" {
		res.SCM.URI = override.SCM.URI
	}
	if override.SCM.Project != "" {
		res.SCM.Project = override.SCM.Project
	}
	if override.SCM.Repo != "" {
		res.SCM.Repo = override.SCM.Repo
	}
	if override.SCM.Owner != "" {
		res.SCM.Owner = override.SCM.Owner
	}

	// Branch
	if override.Branch.Template != "" {
		res.Branch.Template = override.Branch.Template
	}
	if len(override.Branch.Mapping) > 0 {
		if res.Branch.Mapping == nil {
			res.Branch.Mapping = make(map[string]string)
		}
		for k, v := range override.Branch.Mapping {
			res.Branch.Mapping[k] = v
		}
	}

	// Review
	if override.Review.Template != "" {
		res.Review.Template = override.Review.Template
	}

	// Review.Mapping
	if len(override.Review.Mapping.Paths) > 0 {
		if res.Review.Mapping.Paths == nil {
			res.Review.Mapping.Paths = make(map[string]any)
		}
		for k, v := range override.Review.Mapping.Paths {
			res.Review.Mapping.Paths[k] = v
		}
	}
	if len(override.Review.Mapping.Labels) > 0 {
		if res.Review.Mapping.Labels == nil {
			res.Review.Mapping.Labels = make(map[string]any)
		}
		for k, v := range override.Review.Mapping.Labels {
			res.Review.Mapping.Labels[k] = v
		}
	}
	if len(override.Review.Mapping.Branches) > 0 {
		if res.Review.Mapping.Branches == nil {
			res.Review.Mapping.Branches = make(map[string]any)
		}
		for k, v := range override.Review.Mapping.Branches {
			res.Review.Mapping.Branches[k] = v
		}
	}
	if override.Review.Mapping.Default != nil {
		res.Review.Mapping.Default = override.Review.Mapping.Default
	}

	// Review.Merge
	if override.Review.Merge.Instructions != nil {
		res.Review.Merge.Instructions = override.Review.Merge.Instructions
	}
	if override.Review.Merge.Strategy != "" {
		res.Review.Merge.Strategy = override.Review.Merge.Strategy
	}
	if len(override.Review.Merge.Strategies.Sources) > 0 {
		if res.Review.Merge.Strategies.Sources == nil {
			res.Review.Merge.Strategies.Sources = make(map[string]string)
		}
		for k, v := range override.Review.Merge.Strategies.Sources {
			res.Review.Merge.Strategies.Sources[k] = v
		}
	}
	if len(override.Review.Merge.Strategies.Targets) > 0 {
		if res.Review.Merge.Strategies.Targets == nil {
			res.Review.Merge.Strategies.Targets = make(map[string]string)
		}
		for k, v := range override.Review.Merge.Strategies.Targets {
			res.Review.Merge.Strategies.Targets[k] = v
		}
	}

	return res
}

func cloneConfig(c *domain.ProjectConfig) *domain.ProjectConfig {
	if c == nil {
		return nil
	}
	clone := *c

	// Deep copy maps
	if c.Branch.Mapping != nil {
		clone.Branch.Mapping = make(map[string]string, len(c.Branch.Mapping))
		for k, v := range c.Branch.Mapping {
			clone.Branch.Mapping[k] = v
		}
	}
	if c.Review.Mapping.Paths != nil {
		clone.Review.Mapping.Paths = make(map[string]any, len(c.Review.Mapping.Paths))
		for k, v := range c.Review.Mapping.Paths {
			clone.Review.Mapping.Paths[k] = v
		}
	}
	if c.Review.Mapping.Labels != nil {
		clone.Review.Mapping.Labels = make(map[string]any, len(c.Review.Mapping.Labels))
		for k, v := range c.Review.Mapping.Labels {
			clone.Review.Mapping.Labels[k] = v
		}
	}
	if c.Review.Mapping.Branches != nil {
		clone.Review.Mapping.Branches = make(map[string]any, len(c.Review.Mapping.Branches))
		for k, v := range c.Review.Mapping.Branches {
			clone.Review.Mapping.Branches[k] = v
		}
	}
	if c.Review.Merge.Strategies.Sources != nil {
		clone.Review.Merge.Strategies.Sources = make(map[string]string, len(c.Review.Merge.Strategies.Sources))
		for k, v := range c.Review.Merge.Strategies.Sources {
			clone.Review.Merge.Strategies.Sources[k] = v
		}
	}
	if c.Review.Merge.Strategies.Targets != nil {
		clone.Review.Merge.Strategies.Targets = make(map[string]string, len(c.Review.Merge.Strategies.Targets))
		for k, v := range c.Review.Merge.Strategies.Targets {
			clone.Review.Merge.Strategies.Targets[k] = v
		}
	}
	return &clone
}
