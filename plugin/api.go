package plugin

import (
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/jonasbroms/hbm/docker/allow"
	"github.com/jonasbroms/hbm/docker/allow/types"
	"github.com/jonasbroms/hbm/docker/endpoint"
	configobj "github.com/jonasbroms/hbm/object/config"
	groupobj "github.com/jonasbroms/hbm/object/group"
	"github.com/jonasbroms/hbm/pkg/uri"
	"github.com/jonasbroms/hbm/version"
)

type Api struct {
	URIInfo *uri.URIInfo
	Uris    *uri.URIs
	AppPath string
}

func NewApi(uriinfo *uri.URIInfo, appPath string) (*Api, error) {
	uris := endpoint.GetUris()

	return &Api{URIInfo: uriinfo, Uris: uris, AppPath: appPath}, nil
}

func (a *Api) Allow(req authorization.Request) (ar *types.AllowResult) {
	s, err := configobj.New("sqlite", a.AppPath)
	if err != nil {
		slog.Error("Failed to create config object", "version", version.Version, "error", err)
		os.Exit(1)
	}
	defer s.End()

	g, err := groupobj.New("sqlite", a.AppPath)
	if err != nil {
		slog.Error("Failed to create group object", "version", version.Version, "error", err)
		os.Exit(1)
	}
	defer g.End()

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("Recovered panic", "panic", r)
			slog.Warn("Stack trace", "trace", string(debug.Stack()))

			allow, _ := s.Get("default-allow-action-error")
			err := "an error occurred; contact your system administrator"

			result := types.AllowResult{Allow: allow}
			if !allow {
				result.Error = err
			}

			ar = &result
		}
	}()

	// Authentication
	username := req.User
	if len(username) == 0 {
		username = "root"
	}

	// Authorization
	isAdmin := false

	filters := map[string]string{
		"name": "administrators",
		"elem": username,
	}
	groups, _ := g.List(filters)
	if len(groups) > 0 {
		isAdmin = true
	}

	u, err := a.Uris.GetURI(req.RequestMethod, a.URIInfo.Path)
	if err != nil {
		return &types.AllowResult{Allow: false, Error: err.Error()}
	}

	// Validate Docker command is allowed
	aR, _ := s.Get("authorization")
	dOC, _ := s.Get("disable-ownership-check")

	config := types.Config{AppPath: a.AppPath, Username: username, DisableOwnershipCheck: dOC}
	r := allow.True(req, &config)

	if !isAdmin {
		if aR {
			r = allow.Action(&config, u.Action, u.CmdName)
			if r.Allow {
				r = u.AllowFunc(req, &config)
			}
		}
	}

	// Log event with detailed audit information
	if !r.Allow {
		args := []any{
			"event_type", "docker_authorization",
			"user", username,
			"is_admin", isAdmin,
			"allowed", r.Allow,
			"authorization", aR,
			"action", u.Action,
			"command", u.CmdName,
			"request_method", req.RequestMethod,
			"request_uri", req.RequestURI,
			"denial_reason", r.Msg["text"],
		}

		v, ok := r.Msg["resource_type"]
		if ok {
			args = append(args, "resource_type", v)
		}
		v, ok = r.Msg["resource_value"]
		if ok {
			args = append(args, "resource_value", v)
		}

		v, ok = r.Msg["denial_detail"]
		if ok {
			args = append(args, "denial_detail", v)
		}

		// Log denials as warnings for visibility
		slog.Warn("Authorization denied", args...)
	} else {
		// Log allowed actions as info
		slog.Info("Authorization granted",
			"event_type", "docker_authorization",
			"user", username,
			"is_admin", isAdmin,
			"allowed", r.Allow,
			"authorization", aR,
			"action", u.Action,
			"command", u.CmdName,
			"request_method", req.RequestMethod,
			"request_uri", req.RequestURI,
		)
	}

	// If Docker command is not allowed, return
	if !r.Allow {
		return r
	}

	return &types.AllowResult{Allow: true}
}
