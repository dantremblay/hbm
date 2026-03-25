package config

import (
	"fmt"
)

type Config struct {
	Action      string
	Label       string
	Description string
}

type Configs []Config

func New() Configs {
	config := []Config{}

	config = append(config, Config{Action: "authorization", Label: "Authorization", Description: "Authorization"})
	config = append(config, Config{Action: "default-allow-action-error", Label: "Default Allow Action On Error", Description: "Default allow action on error"})
	config = append(config, Config{Action: "disable-ownership-check", Label: "Disable Ownership Check", Description: "Allow users to interact with other users' containers"})

	return config
}

func (c Configs) IsValid(name string) error {
	for _, config := range c {
		if config.Action == name {
			return nil
		}
	}

	return fmt.Errorf("This feature is not valid")
}
