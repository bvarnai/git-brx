package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/ui"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app := cli.NewApp(os.Stdout, os.Stderr, nil)

	if err := app.Execute(ctx); err != nil {
		terminal := ui.New(os.Stdout, os.Stderr, false, false, false)
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			terminal.Error("%s", appErr.Message)
			os.Exit(int(appErr.Code))
		}

		terminal.Error("%v", err)
		os.Exit(int(domain.ExitGeneralError))
	}
}
