package categories

import (
	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	categoriessetorderport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/setorder"
	categoriesreorderproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/reorder"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewReorder builds the reorder use case.
func NewReorder(deps registry.Deps) categoriesreorderproc.Processor {
	return categoriesreorderproc.New(deps.UserTx, categoriesreorderproc.Ports{
		Lock:     categorieslockport.NewPG(),
		List:     categorieslistport.NewPG(),
		SetOrder: categoriessetorderport.NewPG(),
	})
}
