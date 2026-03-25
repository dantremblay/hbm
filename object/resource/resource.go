package resource

import (
	"fmt"
	"strings"

	resourcepkg "github.com/jonasbroms/hbm/docker/resource"
	"github.com/jonasbroms/hbm/internal/image"
	"github.com/jonasbroms/hbm/object/types"
	"github.com/jonasbroms/hbm/storage"
	"github.com/jonasbroms/hbm/storage/driver"
	"github.com/juliengk/go-utils"
	"github.com/juliengk/go-utils/json"
	"github.com/juliengk/go-utils/validation"
)

type Resource interface {
	Add(name, rType, rValue string, rOptions []string) error
	Remove(name string) error
	List(filters map[string]string) (map[types.Resource][]string, error)
	Count(rType string) int
	Find(name string) bool
	AddToCollection(resource, collection string) error
	RemoveFromCollection(resource, collection string) error
	End()
}

type Config struct {
	Storage driver.Storager
}

func New(driver, options string) (Resource, error) {
	s, err := storage.NewDriver(driver, options)
	if err != nil {
		return new(Config), err
	}

	return &Config{
		Storage: s,
	}, nil
}

func (c *Config) End() {
	c.Storage.End()
}

func (c *Config) Add(name, rType, rValue string, rOptions []string) error {
	options := utils.ConvertSliceToMap("=", rOptions)

	if rType == "all" && rValue == "all" {
		return fmt.Errorf("add user to 'administrators' group instead of adding a resource with type and value to 'all'")
	}

	if err := validation.IsValidName(name); err != nil {
		return err
	}

	if c.Storage.FindResource(name) {
		return fmt.Errorf("resource '%s' already exists", name)
	}

	if rType != "all" {
		res, err := resourcepkg.NewDriver(rType)
		if err != nil {
			return err
		}

		if rValue != "all" {
			if err = res.Valid(rValue); err != nil {
				return err
			}
		}

		if err = res.ValidOptions(options); err != nil {
			return err
		}
	}

	if rType == "image" || rType == "plugin" {
		i := image.NewImage(rValue)
		rValue = i.String()
	}

	if rType == "volume" && len(rValue) > 1 {
		rValue = strings.TrimRight(rValue, "/")
	}

	opts := ""
	if rType == "volume" {
		vo := types.VolumeOptions{}
		if _, ok := options["recursive"]; ok {
			vo.Recursive = true
		}
		if _, ok := options["nosuid"]; ok {
			vo.NoSuid = true
		}
		jsonR := json.Encode(vo)
		opts = jsonR.String()
	}

	if rType == "image" {
		io := types.ImageOptions{}
		if _, ok := options["subimages"]; ok {
			io.SubImages = true
			jsonR := json.Encode(io)
			opts = jsonR.String()
			if rValue[len(rValue)-1] != '/' {
				rValue = fmt.Sprintf("%s/", rValue)
			}
		}
	}

	c.Storage.AddResource(name, rType, rValue, strings.TrimSpace(opts))

	return nil
}

func (c *Config) Remove(name string) error {
	if !c.Storage.FindResource(name) {
		return fmt.Errorf("resource '%s' does not exist", name)
	}

	if len(c.Storage.ListCollections(map[string]string{"elem": name})) > 0 {
		return fmt.Errorf("resource '%s' cannot be removed. It is used by at least one collection", name)
	}

	if err := c.Storage.RemoveResource(name); err != nil {
		return err
	}

	return nil
}

func (c *Config) List(filters map[string]string) (map[types.Resource][]string, error) {
	return c.Storage.ListResources(filters), nil
}

func (c *Config) Count(rType string) int {
	return c.Storage.CountResource(rType)
}

func (c *Config) Find(name string) bool {
	filters := map[string]string{"name": name}

	result, err := c.List(filters)
	if err != nil {
		return false
	}

	if len(result) > 0 {
		return true
	}

	return false
}

func (c *Config) AddToCollection(resource, collection string) error {
	if !c.Storage.FindResource(resource) {
		return fmt.Errorf("resource '%s' does not exist", resource)
	}

	if !c.Storage.FindCollection(collection) {
		return fmt.Errorf("collection '%s' does not exist", collection)
	}

	c.Storage.AddResourceToCollection(collection, resource)

	return nil
}

func (c *Config) RemoveFromCollection(resource, collection string) error {
	if !c.Storage.FindResource(resource) {
		return fmt.Errorf("resource '%s' does not exist", resource)
	}

	if !c.Storage.FindCollection(collection) {
		return fmt.Errorf("collection '%s' does not exist", collection)
	}

	c.Storage.RemoveResourceFromCollection(collection, resource)

	return nil
}
