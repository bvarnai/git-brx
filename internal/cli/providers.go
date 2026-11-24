package cli

import (
	"context"
	"strings"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/github"
)

func (a *App) resolveSCM(cfg *domain.ProjectConfig) (domain.SCMProvider, error) {
	provider := strings.ToLower(cfg.SCM.Provider)
	switch provider {
	case "github":
		token := github.ResolveToken(context.Background(), a.Runner, "github.com")
		opts := []github.Option{}
		if cfg.SCM.URI != "" && cfg.SCM.URI != "https://github.com" && cfg.SCM.URI != "http://github.com" {
			opts = append(opts, github.WithBaseURL(cfg.SCM.URI))
		}
		opts = append(opts, github.WithReviewConfig(&cfg.Review))
		return github.NewClient(cfg.SCM.Owner, cfg.SCM.Repo, token, opts...), nil
	case "", "none":
		return nil, nil
	default:
		return nil, nil
	}
}
