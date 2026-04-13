package allow

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/jonasbroms/hbm/docker/allow/types"
	policyobj "github.com/jonasbroms/hbm/object/policy"
	"github.com/jonasbroms/hbm/version"
)

func ContainerOwner(req authorization.Request, config *types.Config) *types.AllowResult {
	if config.DisableOwnershipCheck {
		return &types.AllowResult{Allow: true}
	}

	p, err := policyobj.New("sqlite", config.AppPath)
	if err != nil {
		slog.Error("Failed to create policy object", "version", version.Version, "error", err)
		os.Exit(1)
	}
	defer p.End()

	u, err := url.ParseRequestURI(req.RequestURI)
	if err != nil {
		return &types.AllowResult{
			Allow: false,
			Msg: map[string]string{
				"text": fmt.Sprintf("Failed to parse request URI: %s", err),
			},
		}
	}

	ts := strings.Trim(u.Path, "/")
	up := strings.Split(ts, "/") // api version / type / id
	if len(up) < 3 || up[1] != "containers" {
		return &types.AllowResult{
			Allow: false,
			Msg: map[string]string{
				"text": fmt.Sprintf("Invalid container request URI: %s", u.Path),
			},
		}
	}
	containerID := up[2]
	if !p.ValidateOwner(config.Username, "containers", containerID) {
		return &types.AllowResult{
			Allow: false,
			Msg: map[string]string{
				"text":           fmt.Sprintf("Container %s is not owned by user %s", containerID, config.Username),
				"resource_type":  "container",
				"resource_value": containerID,
			},
		}
	}

	return &types.AllowResult{Allow: true}
}
