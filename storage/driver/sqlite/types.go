package sqlite

import (
	"time"
)

type Model struct {
	ID        uint      `gorm:"primary_key"`
	CreatedAt time.Time `gorm:"created_at"`
}

type AppConfig struct {
	Model
	Key   string `gorm:"unique;"`
	Value bool
}

type User struct {
	Model
	Name string `gorm:"unique;"`
}

type Group struct {
	Model
	Name  string `gorm:"unique;"`
	Users []User `gorm:"many2many:group_users;"`
}

type Resource struct {
	Model
	Name   string `gorm:"unique;"`
	Type   string
	Value  string
	Option string
}

type Collection struct {
	Model
	Name      string     `gorm:"unique;"`
	Resources []Resource `gorm:"many2many:collection_resources;"`
}

type Policy struct {
	Model
	Name         string `gorm:"unique;"`
	Group        Group
	GroupID      uint
	Collection   Collection
	CollectionID uint
}

type ContainerOwner struct {
	Model
	User          User
	UserID        uint
	ContainerID   string `gorm:"unique;"`
	ContainerName string
}

type ContainerOwnerHistory struct {
	Model
	UserID        uint
	Username      string
	ContainerID   string
	ContainerName string
	RemovedAt     time.Time
}

func (ContainerOwnerHistory) TableName() string {
	return "container_owners_history"
}
