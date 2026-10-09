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
