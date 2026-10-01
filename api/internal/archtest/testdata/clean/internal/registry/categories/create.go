package categories

import (
	"example.com/fx/internal/categories/port/insert"
	"example.com/fx/internal/external/mail/send"
)

func NewCreate() { _ = insert.NewPG(); _ = send.NewSMTP() }
