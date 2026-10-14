package config

import (
	"github.com/bvarnai/git-brx/internal/domain"
)

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
