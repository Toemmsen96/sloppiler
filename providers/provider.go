package providers

// Provider is the streaming LLM backend interface used by sloppiler.
// Each implementation handles its own HTTP transport, authentication,
// streaming protocol, and progress step coordination.
type Provider interface {
	// Stream sends prompt to the LLM, drives progressSteps to completion
	// as real streaming milestones are hit (connection established, first token
	// received, generation in progress, stream complete), and returns the full
	// response string. Pass a single-element slice for a simple labelled spinner.
	Stream(prompt string, progressSteps []string) (string, error)
}
