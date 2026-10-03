package categories

import (
	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	categorieslistproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/list"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewList builds the list-categories use case.
func NewList(deps registry.Deps) categorieslistproc.Processor {
	return categorieslistproc.New(deps.UserTx, categorieslistproc.Ports{
		List: categorieslistport.NewPG(),
	})
}
