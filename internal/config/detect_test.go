package config_test

import (
	"testing"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectRemote(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantPlatform string
		wantHost     string
		wantOwner    string
		wantRepo     string
		wantBaseURI  string
	}{
		{
			name:         "GitHub SSH standard",
			url:          "git@github.com:octocat/Hello-World.git",
			wantPlatform: "github",
			wantHost:     "github.com",
			wantOwner:    "octocat",
			wantRepo:     "Hello-World",
			wantBaseURI:  "https://github.com",
		},
		{
			name:         "GitHub HTTPS standard",
			url:          "https://github.com/my-org/backend-service.git",
			wantPlatform: "github",
			wantHost:     "github.com",
			wantOwner:    "my-org",
			wantRepo:     "backend-service",
			wantBaseURI:  "https://github.com",
		},
		{
			name:         "Bitbucket Server SSH with port",
			url:          "ssh://git@bitbucket.company.com:7999/VSB/core-repo.git",
			wantPlatform: "bitbucket",
			wantHost:     "bitbucket.company.com",
			wantOwner:    "VSB",
			wantRepo:     "core-repo",
			wantBaseURI:  "https://bitbucket.company.com",
		},
		{
			name:         "Bitbucket Server HTTPS with scm prefix",
			url:          "https://bitbucket.company.com/scm/VSB/core-repo.git",
			wantPlatform: "bitbucket",
			wantHost:     "bitbucket.company.com",
			wantOwner:    "VSB",
			wantRepo:     "core-repo",
			wantBaseURI:  "https://bitbucket.company.com",
		},
		{
			name:         "GitLab SSH with subgroup",
			url:          "git@gitlab.com:org-group/sub-team/web-app.git",
			wantPlatform: "gitlab",
			wantHost:     "gitlab.com",
			wantOwner:    "org-group/sub-team",
			wantRepo:     "web-app",
			wantBaseURI:  "https://gitlab.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := config.DetectRemote(tt.url)
			require.NotNil(t, res)
			assert.Equal(t, tt.wantPlatform, res.Platform)
			assert.Equal(t, tt.wantHost, res.Host)
			assert.Equal(t, tt.wantOwner, res.Owner)
			assert.Equal(t, tt.wantRepo, res.Repo)
			assert.Equal(t, tt.wantBaseURI, res.BaseURI)
		})
	}

	t.Run("Empty or invalid URL returns nil", func(t *testing.T) {
		assert.Nil(t, config.DetectRemote(""))
		assert.Nil(t, config.DetectRemote("not-a-valid-url"))
	})
}
