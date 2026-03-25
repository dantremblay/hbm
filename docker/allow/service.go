package allow

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/jonasbroms/hbm/docker/allow/types"
	policyobj "github.com/jonasbroms/hbm/object/policy"
	"github.com/jonasbroms/hbm/version"
	"github.com/juliengk/go-utils"
	"github.com/juliengk/go-utils/json"
)

func ServiceCreate(req authorization.Request, config *types.Config) *types.AllowResult {
	svc := &swarm.ServiceSpec{}

	err := json.Decode(req.RequestBody, svc)
	if err != nil {
		return &types.AllowResult{Allow: false, Error: err.Error()}
	}

	defer utils.RecoverFunc()

	p, err := policyobj.New("sqlite", config.AppPath)
	if err != nil {
		slog.Error("Failed to create policy object", "version", version.Version, "error", err)
		os.Exit(1)
	}
	defer p.End()

	if svc.TaskTemplate.ContainerSpec != nil {
		if len(svc.TaskTemplate.ContainerSpec.Mounts) > 0 {
			for _, mount := range svc.TaskTemplate.ContainerSpec.Mounts {
				if mount.Type == "bind" {
					if len(mount.Source) > 0 {
						if allowed, reason := AllowVolume(mount.Source, config); !allowed {
							return &types.AllowResult{
								Allow: false,
								Msg: map[string]string{
									"text":           fmt.Sprintf("Volume %s is not allowed to be mounted", mount.Source),
									"resource_type":  "volume",
									"resource_value": mount.Source,
									"denial_detail":  reason,
								},
							}
						}
					}
				}

				if mount.Type == "tmpfs" {
					if !p.Validate(config.Username, "config", "container_create_param_tmpfs", "") {
						return &types.AllowResult{
							Allow: false,
							Msg: map[string]string{
								"text":           "--tmpfs param is not allowed",
								"resource_type":  "config",
								"resource_value": "container_create_param_tmpfs",
							},
						}
					}
				}
			}
		}

		if len(svc.TaskTemplate.ContainerSpec.User) > 0 {
			if svc.TaskTemplate.ContainerSpec.User == "root" && !p.Validate(config.Username, "config", "container_create_user_root", "") {
				return &types.AllowResult{
					Allow: false,
					Msg: map[string]string{
						"text":           "Running as user \"root\" is not allowed. Please use --user=\"someuser\" param.",
						"resource_type":  "config",
						"resource_value": "container_create_user_root",
					},
				}
			}
		}

		if !AllowImage(svc.TaskTemplate.ContainerSpec.Image, config) {
			return &types.AllowResult{
				Allow: false,
				Msg: map[string]string{
					"text":           fmt.Sprintf("Image %s is not allowed", svc.TaskTemplate.ContainerSpec.Image),
					"resource_type":  "image",
					"resource_value": svc.TaskTemplate.ContainerSpec.Image,
				},
			}
		}
	}

	if svc.TaskTemplate.LogDriver != nil {
		if len(svc.TaskTemplate.LogDriver.Name) > 0 {
			if !p.Validate(config.Username, "logdriver", svc.TaskTemplate.LogDriver.Name, "") {
				return &types.AllowResult{
					Allow: false,
					Msg: map[string]string{
						"text":           fmt.Sprintf("Log driver %s is not allowed", svc.TaskTemplate.LogDriver.Name),
						"resource_type":  "logdriver",
						"resource_value": svc.TaskTemplate.LogDriver.Name,
					},
				}
			}
		}

		if len(svc.TaskTemplate.LogDriver.Options) > 0 {
			for k, v := range svc.TaskTemplate.LogDriver.Options {
				los := fmt.Sprintf("%s=%s", k, v)

				if !p.Validate(config.Username, "logopt", los, "") {
					return &types.AllowResult{
						Allow: false,
						Msg: map[string]string{
							"text":           fmt.Sprintf("Log driver %s is not allowed", los),
							"resource_type":  "logopt",
							"resource_value": los,
						},
					}
				}
			}
		}
	}

	if svc.EndpointSpec != nil {
		if len(svc.EndpointSpec.Ports) > 0 {
			for _, port := range svc.EndpointSpec.Ports {
				if !p.Validate(config.Username, "port", strconv.Itoa(int(port.PublishedPort)), "") {
					return &types.AllowResult{
						Allow: false,
						Msg: map[string]string{
							"text":           fmt.Sprintf("Port %d is not allowed to be published", port.PublishedPort),
							"resource_type":  "port",
							"resource_value": fmt.Sprintf("%d", port.PublishedPort),
						},
					}
				}
			}
		}
	}

	return &types.AllowResult{Allow: true}
}
