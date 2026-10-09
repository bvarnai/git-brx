package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bvarnai/git-brx/internal/git"
)

// ResolveToken retrieves a GitHub personal access token or OAuth token.
// Priority:
// 1. GITHUB_TOKEN environment variable
// 2. GH_TOKEN environment variable
// 3. Git Credential Helper via git.Runner for host
func ResolveToken(ctx context.Context, runner git.Runner, host string) string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token
	}

	if runner == nil {
		return ""
	}

	if host == "" {
		host = "github.com"
	}

	// Consult Git credential helper protocol:
	// echo -e "protocol=https\nhost=github.com\n" | git credential fill
	input := fmt.Sprintf("protocol=https\nhost=%s\n", host)
	out, err := runner.RunWithEnv(ctx, "", []string{}, "credential", "fill")
	if err == nil && out != "" {
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "password=") {
				return strings.TrimPrefix(line, "password=")
			}
		}
	}

	_ = input
	return ""
}
