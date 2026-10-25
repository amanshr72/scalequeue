package processor

import "context"

type Processor interface {
	ProcessJob(context.Context, string, int) error
}
