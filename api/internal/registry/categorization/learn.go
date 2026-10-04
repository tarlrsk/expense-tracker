package categorization

import (
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	categorizationupsertport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/upsert"
	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewLearn builds the learn-merchant-rule use case.
func NewLearn(deps registry.Deps) categorizationlearnproc.Processor {
	return categorizationlearnproc.New(deps.UserTx, categorizationlearnproc.Ports{
		Upsert:   categorizationupsertport.NewPG(),
		Category: categoriesfindport.NewPG(),
	})
}
