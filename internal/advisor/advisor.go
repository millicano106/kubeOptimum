package advisor

import (
	"context"
	"fmt"
)

// Check is implemented by each individual advisor check.
type Check interface {
	Name() string
	Description() string
	Run(ctx context.Context, ac *AnalysisContext) ([]Recommendation, error)
}

// Advisor orchestrates all registered checks against a shared AnalysisContext.
type Advisor struct {
	checks []Check
}

// NewAdvisor creates an Advisor with the provided checks. Checks are run in
// registration order; all checks run even if earlier ones return errors.
func NewAdvisor(checks []Check) *Advisor {
	return &Advisor{checks: checks}
}

// RunResult holds the recommendations and any per-check errors.
type RunResult struct {
	Recommendations []Recommendation
	Errors          []error
}

// Run executes all checks and aggregates their results. Partial results are
// always returned alongside errors so callers can choose how to handle failures.
func (a *Advisor) Run(ctx context.Context, ac *AnalysisContext) RunResult {
	var result RunResult
	for _, check := range a.checks {
		recs, err := check.Run(ctx, ac)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("check %s: %w", check.Name(), err))
			continue
		}
		result.Recommendations = append(result.Recommendations, recs...)
	}
	return result
}
