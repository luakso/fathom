package base

import (
	"context"
	"io"
)

// ProbeDeps bundles the arguments for one probe run.
type ProbeDeps struct {
	Fetcher   Fetcher
	FromBlock uint64
	ToBlock   uint64
}

// ProbeReport summarizes what the probe saw. Detailed fields filled in by
// Task 3.
type ProbeReport struct {
	MatchedEvents int
}

// Print writes the report in a human-readable form.
func (r ProbeReport) Print(_ io.Writer) {}

// RunProbe is a stub until Task 3.
func RunProbe(_ context.Context, _ ProbeDeps) (ProbeReport, error) {
	return ProbeReport{}, nil
}
