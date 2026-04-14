package sqlite

import (
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/jonasbroms/hbm/storage"
	"github.com/jonasbroms/hbm/storage/driver"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func init() {
	storage.RegisterDriver("sqlite", New)
}

type Config struct {
	DB *gorm.DB
}

func New(config string) (driver.Storager, error) {
	debug := false

	file := path.Join(config, "data.db")

	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()

	db, err := gorm.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}

	db.LogMode(debug)

	db.AutoMigrate(&AppConfig{}, &User{}, &Group{}, &Resource{}, &Collection{}, &Policy{}, &ContainerOwner{}, &ContainerOwnerHistory{})

	migrateContainerOwners(db)

	return &Config{DB: db}, nil
}

func migrateContainerOwners(db *gorm.DB) {
	// Check if there are old-style "name:" rows that need migration
	var count int
	db.Model(&ContainerOwner{}).Where("container_id LIKE 'name:%'").Count(&count)
	if count == 0 {
		return
	}

	slog.Info("Migrating container_owners to new schema", "legacy_name_rows", count)

	// For each name: row, find the paired ID row (same user, closest preceding ID)
	// and merge them into a single row with both container_id and container_name
	type legacyRow struct {
		ID            uint
		UserID        uint
		ContainerID   string
		ContainerName string
		CreatedAt     interface{}
	}

	var nameRows []ContainerOwner
	db.Where("container_id LIKE 'name:%'").Find(&nameRows)

	for _, nr := range nameRows {
		containerName := strings.TrimPrefix(nr.ContainerID, "name:")

		// Find the paired ID row: same user, not a name: row, closest ID before this one
		var idRow ContainerOwner
		db.Where("user_id = ? AND container_id NOT LIKE 'name:%' AND id < ?", nr.UserID, nr.ID).Order("id desc").First(&idRow)

		if idRow.ID != 0 {
			// Update the ID row to include the container name
			db.Model(&idRow).Update("container_name", containerName)
		}

		// Delete the name: row
		db.Where("id = ?", nr.ID).Delete(&ContainerOwner{})
	}

	slog.Info("Migration complete")
}

func (c *Config) End() {
	c.DB.Close()
}
