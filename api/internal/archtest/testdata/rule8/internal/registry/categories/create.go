package categories

import "example.com/fx/internal/categories/port/insert"

func NewCreate() { _ = insert.NewPG() }
