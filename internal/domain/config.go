package domain

// ProjectConfig defines the unified application configuration.
type ProjectConfig struct {
	Platform string        `yaml:"platform,omitempty"`
	Tracker  TrackerConfig `yaml:"tracker,omitempty"`
	SCM      SCMConfig     `yaml:"scm,omitempty"`
	Branch   BranchConfig  `yaml:"branch,omitempty"`
	Review   ReviewConfig  `yaml:"review,omitempty"`
}

// TrackerConfig defines issue tracker settings.
type TrackerConfig struct {
	Provider string `yaml:"provider,omitempty"`
	URI      string `yaml:"uri,omitempty"`
	Project  string `yaml:"project,omitempty"`
	Owner    string `yaml:"owner,omitempty"`
	Repo     string `yaml:"repo,omitempty"`
}

// SCMConfig defines source control and pull request platform settings.
type SCMConfig struct {
	Provider string `yaml:"provider,omitempty"`
	URI      string `yaml:"uri,omitempty"`
	Project  string `yaml:"project,omitempty"`
	Repo     string `yaml:"repo,omitempty"`
	Owner    string `yaml:"owner,omitempty"`
}

// BranchConfig defines branch naming contracts and type mapping.
type BranchConfig struct {
	Template string         `yaml:"template,omitempty"`
	Mapping  map[string]any `yaml:"mapping,omitempty"`
}

// ReviewMappingConfig defines structured reviewer routing by paths, labels, branches, and fallback default.
type ReviewMappingConfig struct {
	Paths    map[string]any `yaml:"paths,omitempty"`
	Labels   map[string]any `yaml:"labels,omitempty"`
	Branches map[string]any `yaml:"branches,omitempty"`
	Default  any            `yaml:"default,omitempty"`
}

// ReviewConfig defines code review reviewer routing, description template, and merge configuration.
type ReviewConfig struct {
	Mapping  ReviewMappingConfig `yaml:"mapping,omitempty"`
	Template string              `yaml:"template,omitempty"`
	Merge    MergeConfig         `yaml:"merge,omitempty"`
}

// MergeConfig defines merge instructions, default strategy, and branch-specific strategy mappings.
type MergeConfig struct {
	Instructions *bool           `yaml:"instructions,omitempty"`
	Strategy     string          `yaml:"strategy,omitempty"`
	Strategies   MergeStrategies `yaml:"strategies,omitempty"`
}

// MergeStrategies defines source and target branch pattern overrides for merge strategy.
type MergeStrategies struct {
	Sources map[string]string `yaml:"sources,omitempty"`
	Targets map[string]string `yaml:"targets,omitempty"`
}
