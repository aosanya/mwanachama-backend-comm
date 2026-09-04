package mwanachamacomm

import (
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
)

// TableNames configures which physical tables the stores in this package
// read and write. See [gormstore.TableNames].
type TableNames = gormstore.TableNames

// DefaultTableNames returns comm's eleven production table names. See
// [gormstore.DefaultTableNames].
func DefaultTableNames() TableNames {
	return gormstore.DefaultTableNames()
}

// Migrate creates or updates the tables t names. See [gormstore.Migrate].
func Migrate(db *gorm.DB, t TableNames) error {
	return gormstore.Migrate(db, t)
}
