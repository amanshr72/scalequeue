package processor

import (
	"context"
)

type Processor interface {
	ProcessJob(ctx context.Context, jobID string, streamMsgID string) error
}
