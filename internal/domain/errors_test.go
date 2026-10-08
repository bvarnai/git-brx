package domain_test

import (
	"errors"
	"testing"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAppError(t *testing.T) {
	err := domain.NewError(domain.ExitPreconditionRepo, "not inside repo")
	assert.Equal(t, domain.ExitPreconditionRepo, err.Code)
	assert.Equal(t, "not inside repo", err.Error())

	nested := errors.New("underlying git failure")
	wrapped := domain.WrapError(domain.ExitConflict, nested, "merge conflict detected")
	assert.Equal(t, domain.ExitConflict, wrapped.Code)
	assert.Contains(t, wrapped.Error(), "merge conflict detected: underlying git failure")
	assert.True(t, errors.Is(wrapped, nested))
}
