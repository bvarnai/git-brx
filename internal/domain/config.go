package domain

// ProjectConfig defines the unified application configuration.
type ProjectConfig struct {
	Platform string        `yaml:"platform,omitempty" json:"platform,omitempty"`
	Tracker  TrackerConfig `yaml:"tracker,omitempty" json:"tracker,omitempty"`
	SCM      SCMConfig     `yaml:"scm,omitempty" json:"scm,omitempty"`
	Branch   BranchConfig  `yaml:"branch,omitempty" json:"branch,omitempty"`
	Review   ReviewConfig  `yaml:"review,omitempty" json:"review,omitempty"`
}

// TrackerConfig defines issue tracker settings.
type TrackerConfig struct {
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	URI      string `yaml:"uri,omitempty" json:"uri,omitempty"`
	Project  string `yaml:"project,omitempty" json:"project,omitempty"`
	Owner    string `yaml:"owner,omitempty" json:"owner,omitempty"`
	Repo     string `yaml:"repo,omitempty" json:"repo,omitempty"`
}

// SCMConfig defines source control and pull request platform settings.
type SCMConfig struct {
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	URI      string `yaml:"uri,omitempty" json:"uri,omitempty"`
	Project  string `yaml:"project,omitempty" json:"project,omitempty"`
	Repo     string `yaml:"repo,omitempty" json:"repo,omitempty"`
	Owner    string `yaml:"owner,omitempty" json:"owner,omitempty"`
}

// BranchConfig defines branch naming contracts and type mapping.
type BranchConfig struct {
	Template string            `yaml:"template,omitempty" json:"template,omitempty"`
	Mapping  map[string]string `yaml:"mapping,omitempty" json:"mapping,omitempty"`
}

// ReviewConfig defines code review reviewer routing, description template, and merge configuration.
type ReviewConfig struct {
	Mapping           map[string]any `yaml:"mapping,omitempty" json:"mapping,omitempty"`
	Template          string         `yaml:"template,omitempty" json:"template,omitempty"`
	Merge             MergeConfig    `yaml:"merge,omitempty" json:"merge,omitempty"`
	MergeInstructions *bool          `yaml:"merge_instructions,omitempty" json:"merge_instructions,omitempty"` // Legacy alias for Merge.Instructions
	Instructions      *bool          `yaml:"instructions,omitempty" json:"instructions,omitempty"`             // Legacy alias for Merge.Instructions
}

// MergeConfig defines merge instructions, default strategy, and branch-specific strategy mappings.
type MergeConfig struct {
	Instructions *bool           `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Strategy     string          `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	Strategies   MergeStrategies `yaml:"strategies,omitempty" json:"strategies,omitempty"`
}

// MergeStrategies defines source and target branch pattern overrides for merge strategy.
type MergeStrategies struct {
	Sources map[string]string `yaml:"sources,omitempty" json:"sources,omitempty"`
	Targets map[string]string `yaml:"targets,omitempty" json:"targets,omitempty"`
}
