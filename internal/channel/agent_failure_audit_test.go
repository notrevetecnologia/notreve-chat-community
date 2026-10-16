package channel

import (
	"context"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestProcessorRecordFailureIsSafeWithoutStore(t *testing.T) {
	(&Processor{}).recordFailure(context.Background(), store.AgentFailure{Tipo: store.FailureKindPipeline})
}
