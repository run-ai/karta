package execution

import "context"

// Runner interface for query evaluation against data
//
//go:generate mockgen -source=interface.go -destination=runner_mock.go -package=execution Runner
type Runner interface {
	Evaluate(ctx context.Context, expression string) ([]any, error)
}
