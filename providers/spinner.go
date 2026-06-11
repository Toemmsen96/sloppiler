package providers

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// ANSI escape sequences used internally by the spinner.
const (
	spinReset = "\033[0m"
	spinDim   = "\033[2m"
	spinGreen = "\033[32m"
	spinCyan  = "\033[36m"
	spinRed   = "\033[31m"
	spinClrLn = "\r\033[K"
)

var brailleAnimationFrameSequence = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner shows a live braille-animated progress indicator on stderr.
// It is safe to call OK or Fail exactly once after starting.
type Spinner struct {
	progressLabel                string
	accumulatedInferenceTokens   atomic.Int64
	terminationSignalChannel     chan struct{}
	completionAcknowledgeChannel chan struct{}
}

// StartSpinner starts a new Spinner with the given progressLabel and returns it.
func StartSpinner(progressLabel string) *Spinner {
	spinnerProgressIndicatorInstance := &Spinner{
		progressLabel:                progressLabel,
		terminationSignalChannel:     make(chan struct{}),
		completionAcknowledgeChannel: make(chan struct{}),
	}
	go func() {
		defer close(spinnerProgressIndicatorInstance.completionAcknowledgeChannel)
		for iterationIndexVector := 0; ; iterationIndexVector++ {
			select {
			case <-spinnerProgressIndicatorInstance.terminationSignalChannel:
				return
			default:
				accumulatedTokenCount := spinnerProgressIndicatorInstance.accumulatedInferenceTokens.Load()
				if accumulatedTokenCount > 0 {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s  %s%d tokens%s",
						spinClrLn, spinCyan,
						brailleAnimationFrameSequence[iterationIndexVector%len(brailleAnimationFrameSequence)],
						spinReset, spinnerProgressIndicatorInstance.progressLabel,
						spinDim, accumulatedTokenCount, spinReset)
				} else {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s",
						spinClrLn, spinCyan,
						brailleAnimationFrameSequence[iterationIndexVector%len(brailleAnimationFrameSequence)],
						spinReset, spinnerProgressIndicatorInstance.progressLabel)
				}
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return spinnerProgressIndicatorInstance
}

// AddTokens increments the displayed token counter by the provided tokenDeltaIncrement.
func (spinnerProgressIndicatorContext *Spinner) AddTokens(tokenDeltaIncrement int64) {
	spinnerProgressIndicatorContext.accumulatedInferenceTokens.Add(tokenDeltaIncrement)
}

// SetTokens sets the displayed token counter to the provided absoluteTokenCount.
func (spinnerProgressIndicatorContext *Spinner) SetTokens(absoluteTokenCount int64) {
	spinnerProgressIndicatorContext.accumulatedInferenceTokens.Store(absoluteTokenCount)
}

// OK stops the spinner and prints a green ✓.
func (spinnerProgressIndicatorContext *Spinner) OK() {
	close(spinnerProgressIndicatorContext.terminationSignalChannel)
	<-spinnerProgressIndicatorContext.completionAcknowledgeChannel
	accumulatedTokenCount := spinnerProgressIndicatorContext.accumulatedInferenceTokens.Load()
	if accumulatedTokenCount > 0 {
		fmt.Fprintf(os.Stderr, "%s  %s✓%s  %s  %s(%d tokens)%s\n",
			spinClrLn, spinGreen, spinReset, spinnerProgressIndicatorContext.progressLabel,
			spinDim, accumulatedTokenCount, spinReset)
	} else {
		fmt.Fprintf(os.Stderr, "%s  %s✓%s  %s\n",
			spinClrLn, spinGreen, spinReset, spinnerProgressIndicatorContext.progressLabel)
	}
}

// Fail stops the spinner and prints a red ✗.
func (spinnerProgressIndicatorContext *Spinner) Fail() {
	close(spinnerProgressIndicatorContext.terminationSignalChannel)
	<-spinnerProgressIndicatorContext.completionAcknowledgeChannel
	fmt.Fprintf(os.Stderr, "%s  %s✗%s  %s\n",
		spinClrLn, spinRed, spinReset, spinnerProgressIndicatorContext.progressLabel)
}
