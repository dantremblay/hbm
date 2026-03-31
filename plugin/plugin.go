package plugin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/docker/docker/client"
	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/jonasbroms/hbm/pkg/uri"
	"github.com/jonasbroms/hbm/storage"
)

type plugin struct {
	appPath       string
	skipEndpoints []*regexp.Regexp
}

func stringInRegexpSlice(s string, regexps []*regexp.Regexp) bool {
	for _, re := range regexps {
		if re.MatchString(s) {
			return true
		}
	}

	return false
}

func NewPlugin(appPath string) (*plugin, error) {
	p := plugin{
		appPath: appPath,
		skipEndpoints: []*regexp.Regexp{
			regexp.MustCompile(`^/_ping`),
			regexp.MustCompile(`^/distribution/(.+)/json`),
		},
	}

	return &p, nil
}

func (p *plugin) AuthZReq(req authorization.Request) authorization.Response {
	uriinfo, err := uri.GetURIInfo(req)
	if err != nil {
		return authorization.Response{Err: err.Error()}
	}

	if req.RequestMethod == "OPTIONS" || stringInRegexpSlice(uriinfo.Path, p.skipEndpoints) {
		return authorization.Response{Allow: true}
	}

	a, err := NewApi(&uriinfo, p.appPath)
	if err != nil {
		return authorization.Response{Err: err.Error()}
	}

	r := a.Allow(req)
	if r.Error != "" {
		return authorization.Response{Err: r.Error}
	}
	if !r.Allow {
		return authorization.Response{Msg: r.Msg["text"]}
	}

	return authorization.Response{Allow: true}
}

func (p *plugin) iscreatecontainer(req authorization.Request, u *url.URL) bool {
	if req.ResponseStatusCode != 201 {
		return false
	}
	//fmt.Println("is url:", u)
	avm := regexp.MustCompile("^/v\\d+\\.\\d+/containers/create")
	if avm.MatchString(u.Path) || u.Path == "/containers/create" {
		return true
	}

	return false
}

func (p *plugin) setcontainerowner(cname string, req authorization.Request) error {
	username := req.User
	if username == "" {
		username = "root"
	}

	s, err := storage.NewDriver("sqlite", p.appPath)
	if err != nil {
		return err
	}
	defer s.End()

	var rjson struct {
		Id string
	}
	err = json.Unmarshal(req.ResponseBody, &rjson)
	if err != nil {
		return err
	}

	if cname == "" {
		cname, err = p.getContainerName(rjson.Id)
		if err != nil {
			slog.Warn("Failed to get container name", "container_id", rjson.Id, "error", err)
		}
	}

	s.SetContainerOwner(username, cname, rjson.Id)

	// Audit log for container creation
	slog.Info("Container ownership recorded", "event_type", "container_ownership", "user", username, "container_name", cname, "container_id", rjson.Id)

	return nil
}

func (p *plugin) getContainerName(containerID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(inspect.Name, "/"), nil
}

func (p *plugin) AuthZRes(req authorization.Request) authorization.Response {
	//fmt.Println("resp uri real:", req.RequestURI)
	//fmt.Println("req body:", string(req.RequestBody))
	//fmt.Println("resp body:", string(req.ResponseBody))
	u, err := url.Parse(req.RequestURI)
	if err != nil {
		//fmt.Println("parse error:", err)
		return authorization.Response{Allow: true, Msg: err.Error()}
	}
	//fmt.Println(u)

	cname := u.Query().Get("name")
	if p.iscreatecontainer(req, u) {
		//fmt.Print("setting owner for", cname)
		err = p.setcontainerowner(cname, req)
		//fmt.Println("setcontainterowner err:", err)
	}

	return authorization.Response{Allow: true}
}
