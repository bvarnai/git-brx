package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/github"
	"github.com/spf13/cobra"
)

// PublishOptions stores flags for the publish command.
type PublishOptions struct {
	NoForce bool
	Comment bool
}

// newPublishCmd constructs the 'publish' subcommand.
func (a *App) newPublishCmd() *cobra.Command {
	opts := PublishOptions{}

	cmd := &cobra.Command{
		Use:   "publish [flags]",
		Short: "Pushes current branch to remote with lease and sets upstream tracking",
		Long: `Pushes commits for the current branch to origin using --force-with-lease.
Automatically configures upstream tracking if the branch has not been published yet.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runPublish(cmd.Context(), opts, args)
		},
	}

	cmd.Flags().BoolVar(&opts.NoForce, "no-force", false, "Push with standard fast-forward constraints without --force-with-lease")
	cmd.Flags().BoolVarP(&opts.Comment, "comment", "c", false, "Post a comment linking to the branch on the associated issue")

	return cmd
}

func (a *App) runPublish(ctx context.Context, opts PublishOptions, args []string) error {
	if len(args) > 0 {
		return domain.NewError(domain.ExitUsageError, "Unexpected argument: %s", args[0])
	}

	workDir := "."
	if a.WorkingDir != "" {
		workDir = a.WorkingDir
	}

	rootDir, err := checks.InsideWorkTree(ctx, a.Inspector, workDir)
	if err != nil {
		return err
	}

	if err := checks.NotShallow(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	if err := checks.NoActiveOperation(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	currBranch, err := checks.AttachedBranch(ctx, a.Inspector, rootDir)
	if err != nil {
		return err
	}

	if err := checks.HasOrigin(ctx, a.Inspector, rootDir); err != nil {
		return err
	}

	// Protected branch safety assertion
	if (currBranch.Name == "master" || currBranch.Name == "main") && !opts.NoForce {
		return domain.NewError(domain.ExitUsageError, "Refusing to force-push with lease directly to protected '%s' branch", currBranch.Name).
			WithHint("Use standard push for default branches or switch to a topic branch")
	}

	// Check upstream tracking association
	expectedUpstream := "origin/" + currBranch.Name
	setUpstream := currBranch.UpstreamRef != expectedUpstream

	forceWithLease := !opts.NoForce

	// Dry run mode
	if a.Opts.DryRun {
		_, pushErr := a.Operations.Push(ctx, rootDir, "origin", currBranch.Name, setUpstream, forceWithLease, true)
		if pushErr != nil {
			return a.handlePushError(pushErr)
		}
		a.UI.Log("Would publish branch '%s' to origin", currBranch.Name)
		return nil
	}

	// Execute push
	_, pushErr := a.Operations.Push(ctx, rootDir, "origin", currBranch.Name, setUpstream, forceWithLease, false)
	if pushErr != nil {
		return a.handlePushError(pushErr)
	}

	a.UI.Log("Branch '%s' successfully published to origin", currBranch.Name)

	if opts.Comment {
		a.postBranchCommentToIssue(ctx, rootDir, currBranch.Name)
	}

	return nil
}

func (a *App) postBranchCommentToIssue(ctx context.Context, rootDir, branchName string) {
	hasOrigin, originURL, err := a.Inspector.HasOrigin(ctx, rootDir)
	if err != nil || !hasOrigin {
		return
	}

	cfg, err := config.Load(rootDir, originURL)
	if err != nil || cfg == nil {
		return
	}

	templatePattern := cfg.Branch.Template
	if templatePattern == "" {
		templatePattern = `^(issue|feature|epic)/([A-Za-z0-9_-]+)$`
	}
	re, _ := regexp.Compile(templatePattern)
	var issueKey string
	if re != nil {
		matches := re.FindStringSubmatch(branchName)
		if len(matches) >= 3 {
			issueKey = matches[2]
		}
	}
	if issueKey == "" {
		parts := strings.SplitN(branchName, "/", 2)
		if len(parts) == 2 {
			issueKey = parts[1]
		}
	}
	if issueKey == "" {
		return
	}

	token := github.ResolveToken(ctx, a.Runner, "github.com")
	if token == "" {
		return
	}

	opts := []github.Option{}
	if cfg.Tracker.URI != "" && cfg.Tracker.URI != "https://github.com" && cfg.Tracker.URI != "http://github.com" {
		opts = append(opts, github.WithBaseURL(cfg.Tracker.URI))
	}
	client := github.NewClient(cfg.Tracker.Owner, cfg.Tracker.Repo, token, opts...)

	branchURL := fmt.Sprintf("https://github.com/%s/%s/tree/%s", cfg.SCM.Owner, cfg.SCM.Repo, branchName)
	commentBody := fmt.Sprintf("Working branch: [%s](%s)", branchName, branchURL)

	if err := client.AddIssueComment(ctx, issueKey, commentBody); err == nil {
		a.UI.Log("Added branch reference comment to issue #%s", issueKey)
	}
}

func (a *App) handlePushError(pushErr error) error {
	errMsg := strings.ToLower(pushErr.Error())

	// Stale lease or non-fast-forward rejection
	if strings.Contains(errMsg, "stale info") ||
		strings.Contains(errMsg, "[rejected") ||
		strings.Contains(errMsg, "fetch first") ||
		strings.Contains(errMsg, "non-fast-forward") {
		return domain.NewError(domain.ExitPreconditionRemote, "Publish rejected: remote has newer commits").
			WithHint("Run 'git-brx update' to incorporate remote changes before publishing")
	}

	return domain.NewError(domain.ExitPreconditionRemote, "Publish failed: unable to reach remote or authentication rejected")
}
