package categories

import (
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	categoriesupdateport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/update"
	categoriesupdateproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/update"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewUpdate builds the update-category use case (rename, icon, archive, unarchive).
func NewUpdate(deps registry.Deps) categoriesupdateproc.Processor {
	return categoriesupdateproc.New(deps.UserTx, categoriesupdateproc.Ports{
		Lock:   categorieslockport.NewPG(),
		Find:   categoriesfindport.NewPG(),
		Update: categoriesupdateport.NewPG(),
	})
}
