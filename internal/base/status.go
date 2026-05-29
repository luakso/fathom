package base

import (
	"context"
	"io"
)

// StatusReport summarizes collector health. Detailed fields filled in by Task 4.
type StatusReport struct {
	Cursor uint64
}

// Print writes the report in a human-readable form.
func (r StatusReport) Print(_ io.Writer) {}

// RunStatus is a stub until Task 4.
func RunStatus(_ context.Context, _ *Store) (StatusReport, error) {
	return StatusReport{}, nil
}
