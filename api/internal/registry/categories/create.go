package categories

import (
	categoriescountport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/count"
	categoriesinsertport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/insert"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	categoriescreateproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/create"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewCreate builds the create-category use case.
func NewCreate(deps registry.Deps) categoriescreateproc.Processor {
	return categoriescreateproc.New(deps.UserTx, categoriescreateproc.Ports{
		Lock:   categorieslockport.NewPG(),
		Count:  categoriescountport.NewPG(),
		Insert: categoriesinsertport.NewPG(),
	})
}
