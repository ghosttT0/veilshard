package rollback

import (
	"context"
	"fmt"
)

type Step struct {
	Description string
	Undo        func(ctx context.Context) error
}

type Engine struct {
	steps []Step
}

func New() *Engine {
	return &Engine{
		steps: make([]Step, 0),
	}
}

// Push registers a compensational undo action to execute if failure occurs.
func (e *Engine) Push(description string, undo func(ctx context.Context) error) {
	e.steps = append(e.steps, Step{
		Description: description,
		Undo:        undo,
	})
}

// Rollback executes all registered undo actions in LIFO (reverse) order.
func (e *Engine) Rollback(ctx context.Context, logger func(msg string, success bool)) {
	if len(e.steps) == 0 {
		return
	}

	for i := len(e.steps) - 1; i >= 0; i-- {
		step := e.steps[i]
		if step.Undo == nil {
			continue
		}

		err := step.Undo(ctx)
		if logger != nil {
			if err != nil {
				logger(fmt.Sprintf("%s (failed: %v)", step.Description, err), false)
			} else {
				logger(step.Description, true)
			}
		}
	}
}

// Clear clears all undo steps when operation completes successfully.
func (e *Engine) Clear() {
	e.steps = nil
}
