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

func TestFormatDefaultMergeInstructions_MergeStrategy(t *testing.T) {
	tests := []struct {
		name            string
		opts            domain.MergeInstructionOptions
		expectedCommit  string
		unexpectedMatch string
	}{
		{
			name: "strips conventional epic(#91) prefix from PRTitle",
			opts: domain.MergeInstructionOptions{
				PRTitle:      "epic(#91): workflow hardening",
				Branch:       "epic/91",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'epic/91' into main: workflow hardening`",
		},
		{
			name: "strips conventional feat(VSB-101) prefix from PRTitle",
			opts: domain.MergeInstructionOptions{
				PRTitle:      "feat(VSB-101): support OAuth2",
				Branch:       "feature/VSB-101",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'feature/VSB-101' into main: support OAuth2`",
		},
		{
			name: "preserves custom non-conventional PRTitle",
			opts: domain.MergeInstructionOptions{
				PRTitle:      "Custom epic title",
				Branch:       "epic/91",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'epic/91' into main: Custom epic title`",
		},
		{
			name: "falls back to Summary and strips conventional prefix when PRTitle empty",
			opts: domain.MergeInstructionOptions{
				Summary:      "epic(workflow): workflow hardening",
				Branch:       "epic/91",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'epic/91' into main: workflow hardening`",
		},
		{
			name: "uses plain Summary when PRTitle empty",
			opts: domain.MergeInstructionOptions{
				Summary:      "My summary",
				Branch:       "epic/1",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'epic/1' into main: My summary`",
		},
		{
			name: "strips #IssueKey: prefix from PRTitle",
			opts: domain.MergeInstructionOptions{
				PRTitle:      "#91: Workflow hardening",
				IssueKey:     "91",
				Branch:       "epic/91",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit: "`Merge branch 'epic/91' into main: Workflow hardening`",
		},
		{
			name: "omits suffix when PRTitle equals Branch and Summary is empty",
			opts: domain.MergeInstructionOptions{
				PRTitle:      "epic/91",
				Branch:       "epic/91",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit:  "`Merge branch 'epic/91' into main`",
			unexpectedMatch: "main:",
		},
		{
			name: "omits suffix when both PRTitle and Summary are empty",
			opts: domain.MergeInstructionOptions{
				Branch:       "epic/1",
				TargetBranch: "main",
				Strategy:     "merge",
			},
			expectedCommit:  "`Merge branch 'epic/1' into main`",
			unexpectedMatch: "main:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := domain.FormatDefaultMergeInstructions(tt.opts)
			assert.Contains(t, out, tt.expectedCommit)
			if tt.unexpectedMatch != "" {
				assert.NotContains(t, out, tt.unexpectedMatch)
			}
		})
	}
}
