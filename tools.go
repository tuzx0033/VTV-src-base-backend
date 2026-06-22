//go:build tools

// Package tools pins build-time CLI tools so they're tracked in go.mod and
// reproducibly installable. Not compiled into any binary.
package tools

import (
	_ "github.com/golang-migrate/migrate/v4/cmd/migrate"
	_ "github.com/swaggo/swag/cmd/swag"
)
