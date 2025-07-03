package postgres

import (
	"gorm.io/gorm"
)

type Postgres interface {
	Connect(cfg DBConfig) (*gorm.DB, error)
	GetDB() *gorm.DB
	RunMigrations(cfg DBConfig, migrationsDir string) error
}
