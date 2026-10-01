package check

import (
	"context"
	"fmt"

	healthpingport "github.com/tarlrsk/expense-tracker/api/internal/health/port/ping"
)

type processor struct {
	ping healthpingport.Port
}

// New returns the liveness processor.
func New(ping healthpingport.Port) Processor {
	return &processor{ping: ping}
}

func (p *processor) Execute(ctx context.Context, _ Request) (Response, error) {
	if err := p.ping.Ping(ctx); err != nil {
		return Response{}, fmt.Errorf("health check: ping: %w", err)
	}
	return Response{}, nil
}
