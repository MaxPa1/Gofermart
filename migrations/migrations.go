// Package migrations содержит SQL-миграции схемы базы данных в формате goose, вшитые в бинарь.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
