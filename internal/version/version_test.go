package version_test

import (
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/version"
	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	info := version.Get()
	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.GoVersion)
	assert.NotEmpty(t, info.Platform)
}

func TestString(t *testing.T) {
	s := version.String()
	assert.True(t, strings.HasPrefix(s, "git-brx version "))
	assert.Contains(t, s, "commit:")
	assert.Contains(t, s, "built at:")
}

func TestGetWithInjectedCommit(t *testing.T) {
	origCommit := version.Commit
	origVersion := version.Version
	origDate := version.Date
	defer func() {
		version.Commit = origCommit
		version.Version = origVersion
		version.Date = origDate
	}()

	version.Version = "v1.0.3"
	version.Commit = "4900e38"
	version.Date = "2026-10-12T09:25:38Z"

	info := version.Get()
	assert.Equal(t, "v1.0.3", info.Version)
	assert.Equal(t, "4900e38", info.Commit)
	assert.False(t, strings.HasSuffix(info.Commit, "-dirty"), "Injected commit should not have -dirty suffix")
	assert.Equal(t, "2026-10-12T09:25:38Z", info.Date)

	formatted := version.String()
	assert.Contains(t, formatted, "commit: 4900e38,")
	assert.NotContains(t, formatted, "4900e38-dirty")
}
