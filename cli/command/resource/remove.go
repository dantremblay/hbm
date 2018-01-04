package resource

import (
	"github.com/juliengk/go-utils"
	"github.com/kassisol/hbm/cli/command"
	resourceobj "github.com/kassisol/hbm/object/resource"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm [name]",
		Aliases: []string{"remove"},
		Short:   "Remove resource from the whitelist",
		Long:    removeDescription,
		Args:    cobra.ExactArgs(1),
		Run:     runRemove,
	}

	return cmd
}

func runRemove(cmd *cobra.Command, args []string) {
	defer utils.RecoverFunc()

	r, err := resourceobj.New("sqlite", command.AppPath)
	if err != nil {
		log.Fatal(err)
	}
	defer r.End()

	if err := r.Remove(args[0]); err != nil {
		log.Fatal(err)
	}
}

var removeDescription = `
Remove resource from the whitelist

`
