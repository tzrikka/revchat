package temporal

import (
	"fmt"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// GetSignalChannels marks the calling workflow as a receiver for
// the given signals, and returns Go channels corresonding to them.
func GetSignalChannels(ctx workflow.Context, signals []string) ([]workflow.ReceiveChannel, error) {
	// https://docs.temporal.io/develop/go/observability#visibility
	sa := temporal.NewSearchAttributeKeyKeywordList("WaitingForSignals").ValueSet(signals)
	if err := workflow.UpsertTypedSearchAttributes(ctx, sa); err != nil {
		return nil, fmt.Errorf("failed to set workflow search attribute: %w", err)
	}

	ch := make([]workflow.ReceiveChannel, len(signals))
	for i, sig := range signals {
		ch[i] = workflow.GetSignalChannel(ctx, sig)
	}

	return ch, nil
}
