package config

import (
	"github.com/juliengk/go-utils"
	"github.com/kassisol/hbm/cli/command"
	configobj "github.com/kassisol/hbm/object/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set HBM config option",
		Long:  setDescription,
		Args:  cobra.ExactArgs(2),
		Run:   runSet,
	}

	return cmd
}

func runSet(cmd *cobra.Command, args []string) {
	defer utils.RecoverFunc()

	c, err := configobj.New("sqlite", command.AppPath)
	if err != nil {
		log.Fatal(err)
	}
	defer c.End()

	if err := c.Set(args[0], args[1]); err !=nil {
		log.Fatal(err)
	}
}

var setDescription = `
Set HBM config option

`
