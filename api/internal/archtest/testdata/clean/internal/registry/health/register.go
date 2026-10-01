package health

import (
	healthpingport "example.com/fx/internal/health/port/ping"
	healthcheckproc "example.com/fx/internal/health/processor/check"
	"example.com/fx/internal/registry"
)

func NewCheck() healthcheckproc.Processor { return healthcheckproc.New(healthpingport.NewStatic()) }
