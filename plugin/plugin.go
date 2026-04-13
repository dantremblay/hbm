package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/jonasbroms/hbm/pkg/uri"
	"github.com/jonasbroms/hbm/storage"
)

type containerInfo struct {
	ID     string
	Name   string
	AutoRM bool
}

type plugin struct {
	appPath            string
	skipEndpoints      []*regexp.Regexp
	internalContainers map[string]bool
	pendingRemovals    map[string]*containerInfo
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
		internalContainers: make(map[string]bool),
		pendingRemovals:    make(map[string]*containerInfo),
	}

	go p.purgeStaleOwners()

	return &p, nil
}

func (p *plugin) waitForDocker() bool {
	httpc := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	for i := 0; i < 30; i++ {
		resp, err := httpc.Get("http://localhost/_ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func (p *plugin) purgeStaleOwners() {
	if !p.waitForDocker() {
		slog.Warn("Docker not available, skipping stale container owner purge")
		return
	}

	s, err := storage.NewDriver("sqlite", p.appPath)
	if err != nil {
		slog.Warn("Failed to open storage for stale owner purge", "error", err)
		return
	}
	defer s.End()

	httpc := http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	containerIDs := s.ListContainerOwnerIDs()

	// Mark all IDs as internal so our inspect calls bypass AuthZReq
	for _, cid := range containerIDs {
		p.internalContainers[cid] = true
	}

	var purged int
	for _, cid := range containerIDs {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost/containers/%s/json", cid))
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			s.RemoveContainerOwner(cid)
			purged++
		}
	}

	// Clean up orphaned name: rows whose container ID rows were just removed
	orphaned := s.RemoveOrphanedContainerOwnerNames()
	purged += orphaned

	// Clean up internal markers
	for _, cid := range containerIDs {
		delete(p.internalContainers, cid)
	}

	if purged > 0 {
		slog.Info("Purged stale container ownership entries", "count", purged)
	}
}

func (p *plugin) AuthZReq(req authorization.Request) authorization.Response {
	uriinfo, err := uri.GetURIInfo(req)
	if err != nil {
		return authorization.Response{Err: err.Error()}
	}

	if req.RequestMethod == "OPTIONS" || stringInRegexpSlice(uriinfo.Path, p.skipEndpoints) {
		return authorization.Response{Allow: true}
	}

	if req.RequestMethod == "GET" {
		re := regexp.MustCompile(`^/containers/(.+)/json$`)
		if m := re.FindStringSubmatch(uriinfo.Path); m != nil {
			if p.internalContainers[m[1]] {
				return authorization.Response{Allow: true}
			}
		}
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

	// Capture container info before removal/stop/kill so we can clean up ownership in AuthZRes
	if req.RequestMethod == "DELETE" || req.RequestMethod == "POST" {
		var re *regexp.Regexp
		switch req.RequestMethod {
		case "DELETE":
			re = regexp.MustCompile(`^/containers/([^/]+)$`)
		case "POST":
			re = regexp.MustCompile(`^/containers/([^/]+)/(?:stop|kill)$`)
		}
		if re != nil {
			if m := re.FindStringSubmatch(uriinfo.Path); m != nil {
				p.captureContainerInfo(m[1])
			}
		}
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
		p.internalContainers[rjson.Id] = true
		cname, err = p.getContainerName(rjson.Id)
		delete(p.internalContainers, rjson.Id)
		if err != nil {
			slog.Warn("Failed to get container name", "container_id", rjson.Id, "error", err)
		}
	}

	s.SetContainerOwner(username, cname, rjson.Id)

	// Audit log for container creation
	slog.Info("Container ownership recorded", "event_type", "container_ownership", "user", username, "container_name", cname, "container_id", rjson.Id)

	return nil
}

func (p *plugin) captureContainerInfo(containerRef string) {
	p.internalContainers[containerRef] = true
	defer delete(p.internalContainers, containerRef)

	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	info := &containerInfo{ID: containerRef}

	resp, err := httpc.Get(fmt.Sprintf("http://localhost/containers/%s/json", containerRef))
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var result struct {
				ID         string `json:"Id"`
				Name       string `json:"Name"`
				HostConfig struct {
					AutoRemove bool `json:"AutoRemove"`
				} `json:"HostConfig"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				if result.ID != "" {
					info.ID = result.ID
				}
				info.Name = strings.TrimPrefix(result.Name, "/")
				info.AutoRM = result.HostConfig.AutoRemove
			}
		}
	}

	p.pendingRemovals[containerRef] = info
}

func (p *plugin) removecontainerowner(containerRef string, isDelete bool) {
	info, ok := p.pendingRemovals[containerRef]
	if !ok {
		return
	}
	delete(p.pendingRemovals, containerRef)

	// For stop/kill, only clean up if the container was started with --rm
	if !isDelete && !info.AutoRM {
		return
	}

	s, err := storage.NewDriver("sqlite", p.appPath)
	if err != nil {
		slog.Warn("Failed to open storage for container owner removal", "error", err)
		return
	}
	defer s.End()

	s.RemoveContainerOwner(info.ID)
	if info.Name != "" {
		s.RemoveContainerOwner(info.Name)
	}

	slog.Info("Container ownership removed", "event_type", "container_ownership", "container_id", info.ID, "container_name", info.Name)
}

func (p *plugin) getContainerName(containerID string) (string, error) {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	resp, err := httpc.Get(fmt.Sprintf("http://localhost/containers/%s/json", containerID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("inspect returned status %d", resp.StatusCode)
	}

	var result struct {
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return strings.TrimPrefix(result.Name, "/"), nil
}

// isremovecontainer returns (matched, containerRef, isDelete)
func (p *plugin) isremovecontainer(req authorization.Request, u *url.URL) (bool, string, bool) {
	if req.ResponseStatusCode != 204 {
		return false, "", false
	}
	// Explicit DELETE /containers/{id}
	re := regexp.MustCompile(`^(?:/v\d+\.\d+)?/containers/([^/]+)$`)
	if m := re.FindStringSubmatch(u.Path); m != nil {
		return true, m[1], true
	}
	// POST /containers/{id}/stop or /kill — container may have been auto-removed (--rm)
	re = regexp.MustCompile(`^(?:/v\d+\.\d+)?/containers/([^/]+)/(?:stop|kill)$`)
	if m := re.FindStringSubmatch(u.Path); m != nil {
		return true, m[1], false
	}
	return false, "", false
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
		err = p.setcontainerowner(cname, req)
	}

	if ok, containerRef, isDelete := p.isremovecontainer(req, u); ok {
		p.removecontainerowner(containerRef, isDelete)
	}

	return authorization.Response{Allow: true}
}
