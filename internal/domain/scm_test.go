package domain_test

import (
	"testing"

	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectPullRequest(t *testing.T) {
	tests := []struct {
		name       string
		candidates []domain.PullRequestDetail
		expected   *domain.PullRequestDetail
	}{
		{
			name:       "empty candidates returns nil",
			candidates: nil,
			expected:   nil,
		},
		{
			name: "single PR returns copy",
			candidates: []domain.PullRequestDetail{
				{ID: "5", Number: 5, State: domain.PRStateClosed, URL: "https://example.com/pull/5"},
			},
			expected: &domain.PullRequestDetail{ID: "5", Number: 5, State: domain.PRStateClosed, URL: "https://example.com/pull/5"},
		},
		{
			name: "open takes priority over merged and closed",
			candidates: []domain.PullRequestDetail{
				{ID: "10", Number: 10, State: domain.PRStateClosed},
				{ID: "20", Number: 20, State: domain.PRStateMerged, Merged: true},
				{ID: "15", Number: 15, State: domain.PRStateOpen},
			},
			expected: &domain.PullRequestDetail{ID: "15", Number: 15, State: domain.PRStateOpen},
		},
		{
			name: "merged takes priority over closed",
			candidates: []domain.PullRequestDetail{
				{ID: "30", Number: 30, State: domain.PRStateClosed},
				{ID: "12", Number: 12, State: domain.PRStateMerged, Merged: true},
			},
			expected: &domain.PullRequestDetail{ID: "12", Number: 12, State: domain.PRStateMerged, Merged: true},
		},
		{
			name: "same state breaks tie by highest PR number",
			candidates: []domain.PullRequestDetail{
				{ID: "12", Number: 12, State: domain.PRStateMerged, Merged: true},
				{ID: "25", Number: 25, State: domain.PRStateMerged, Merged: true},
				{ID: "18", Number: 18, State: domain.PRStateMerged, Merged: true},
			},
			expected: &domain.PullRequestDetail{ID: "25", Number: 25, State: domain.PRStateMerged, Merged: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := domain.SelectPullRequest(tt.candidates)
			if tt.expected == nil {
				assert.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			assert.Equal(t, *tt.expected, *actual)
		})
	}
}
