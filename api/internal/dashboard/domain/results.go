package domain

import "github.com/google/uuid"

// TrendMonths is how many months the trend covers: the chosen month and the five before it
// (ADR-0088).
const TrendMonths = 6

// TopDescriptionsLimit is how many rows the top descriptions list has at most (ADR-0088).
const TopDescriptionsLimit = 5

// MonthTotals is what a scope spent and received in one month. Income or expense comes from the
// category's kind; archived categories count like any other.
type MonthTotals struct {
	Spent  Money
	Income Money
}

// Difference is income minus spent; negative when more was spent than received.
func (t MonthTotals) Difference() Money { return t.Income - t.Spent }

// CategoryTotal is one expense category's spending in a month.
type CategoryTotal struct {
	CategoryID uuid.UUID
	Name       string
	Icon       string
	// Archived is true for an archived category that still has spending in the month.
	Archived bool
	Total    Money
	// Count is the number of transactions.
	Count int
}

// MonthSpending is the spending of one month of the trend; zero for a month without spending.
type MonthSpending struct {
	Month Month
	Spent Money
}

// DescriptionTotal is one row of the top descriptions: the spending under one description
// (grouped ignoring upper and lower case and the spaces at its ends, ADR-0088).
type DescriptionTotal struct {
	// Description is the spelling used most in the group that month, without outer spaces.
	Description string
	Total       Money
	// Count is the number of transactions.
	Count int
}
