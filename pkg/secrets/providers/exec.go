package providers

import (
	"context"
	"fmt"
	"strings"

	"memdoor/pkg/shell"
)

type execProvider struct{}

func NewExecProvider() *execProvider {
	return &execProvider{}
}

func (p *execProvider) Name() string { return "exec" }

func (p *execProvider) Get(ctx context.Context, id string) (string, error) {
	cmd := shell.Command(ctx, id)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute secret command %q: %w", id, err)
	}
	return strings.TrimRight(string(out), "\n\r"), nil
}

func (p *execProvider) Set(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("exec provider does not support Set")
}

func (p *execProvider) Delete(_ context.Context, _ string) error {
	return fmt.Errorf("exec provider does not support Delete")
}

func (p *execProvider) List(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("exec provider does not support List")
}
