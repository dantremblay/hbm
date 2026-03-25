package system

import (
	"log/slog"
	"os"
	"reflect"

	"github.com/jonasbroms/hbm/docker/endpoint"
	resourcepkg "github.com/jonasbroms/hbm/docker/resource"
	rconfigdrv "github.com/jonasbroms/hbm/docker/resource/driver/config"
	configobj "github.com/jonasbroms/hbm/object/config"
	groupobj "github.com/jonasbroms/hbm/object/group"
	resourceobj "github.com/jonasbroms/hbm/object/resource"
	"github.com/jonasbroms/hbm/pkg/adf"
	"github.com/juliengk/go-utils"
	"github.com/juliengk/go-utils/filedir"
	"github.com/spf13/cobra"
)

var (
	initAction bool
	initConfig bool
)

func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize config",
		Long:  initDescription,
		Args:  cobra.NoArgs,
		Run:   runInit,
	}

	flags := cmd.Flags()
	flags.BoolVarP(&initAction, "action", "", false, "Initialize action resources")
	flags.BoolVarP(&initConfig, "config", "", false, "Initialize config resources")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) {
	if err := filedir.CreateDirIfNotExist(adf.AppPath, false, 0700); err != nil {
		slog.Error("Failed to create application directory", "error", err)
		os.Exit(1)
	}

	s, err := configobj.New("sqlite", adf.AppPath)
	if err != nil {
		slog.Error("Failed to initialize config store", "error", err)
		os.Exit(1)
	}
	defer s.End()

	config, err := s.List(map[string]string{})
	if err != nil {
		slog.Error("Failed to list configs", "error", err)
		os.Exit(1)
	}

	configDefaults := map[string]string{
		"authorization":              "false",
		"default-allow-action-error": "false",
		"disable-ownership-check":    "false",
	}
	existing := make(map[string]bool, len(config))
	for _, c := range config {
		existing[c.Key] = true
	}
	for key, val := range configDefaults {
		if !existing[key] {
			s.Set(key, val)
		}
	}

	g, err := groupobj.New("sqlite", adf.AppPath)
	if err != nil {
		slog.Error("Failed to initialize group store", "error", err)
		os.Exit(1)
	}
	defer g.End()

	filters := map[string]string{
		"name": "administrators",
	}
	groups, _ := g.List(filters)
	if len(groups) == 0 {
		g.Add("administrators")
	}

	r, err := resourceobj.New("sqlite", adf.AppPath)
	if err != nil {
		slog.Error("Failed to initialize resource store", "error", err)
		os.Exit(1)
	}
	defer r.End()

	if initAction {
		for _, u := range *endpoint.GetUris() {
			if !r.Find(u.Action) {
				if err := r.Add(u.Action, "action", u.Action, []string{}); err != nil {
					slog.Error("Failed to add action resource", "error", err)
					os.Exit(1)
				}
			}
		}
	}

	if initConfig {
		res, err := resourcepkg.NewDriver("config")
		if err != nil {
			slog.Error("Failed to create config driver", "error", err)
			os.Exit(1)
		}

		val := utils.GetReflectValue(reflect.Slice, res.List())
		v := val.Interface().([]rconfigdrv.Action)

		for _, c := range v {
			if !r.Find(c.Key) {
				if err := r.Add(c.Key, "config", c.Key, []string{}); err != nil {
					slog.Error("Failed to add config resource", "error", err)
					os.Exit(1)
				}
			}
		}
	}
}

var initDescription = `
Initialize config

`
