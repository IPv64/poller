package poller

/*
	I consider this file the pinacle example of how to allow a Go application to be configured from a file.
	You can put your configuration into any file format: XML, YAML, JSON, TOML, and you can override any
	struct member using an environment variable. The Duration type is also supported. All of the Config{}
	and Duration{} types and methods are reusable in other projects. Just adjust the data in the struct to
	meet your app's needs. See the New() procedure and Start() method in start.go for example usage.
*/

import (
	"os"
	"path"
	"plugin"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/unifi-poller/unifi"
	"golift.io/cnfg"
	"golift.io/cnfg/cnfgfile"
)

const (
	// AppName is the name of the application.
	AppName = "unifi-poller"
	// ENVConfigPrefix is the prefix appended to an env variable tag name.
	ENVConfigPrefix = "UP"
)

// UnifiPoller contains the application startup data, and auth info for UniFi & Influx.
type UnifiPoller struct {
	Flags *Flags
	*Config
}

// Flags represents the CLI args available and their settings.
type Flags struct {
	ConfigFile string
	DumpJSON   string
	ShowVer    bool
	*pflag.FlagSet
}

// Metrics is a type shared by the exporting and reporting packages.
type Metrics struct {
	TS time.Time
	unifi.Sites
	unifi.IDSList
	unifi.Clients
	*unifi.Devices
	SitesDPI   []*unifi.DPITable
	ClientsDPI []*unifi.DPITable
}

// Config represents the core library input data.
type Config struct {
	*Poller `json:"poller" toml:"poller" xml:"poller" yaml:"poller"`
}

// Poller is the global config values.
type Poller struct {
	Plugins []string `json:"plugins" toml:"plugins" xml:"plugin" yaml:"plugins"`
	Debug   bool     `json:"debug" toml:"debug" xml:"debug,attr" yaml:"debug"`
	Quiet   bool     `json:"quiet,omitempty" toml:"quiet,omitempty" xml:"quiet,attr" yaml:"quiet"`
}

// LoadPlugins reads-in dynamic shared libraries.
// Not used very often, if at all.
func (u *UnifiPoller) LoadPlugins() error {
	for _, p := range u.Plugins {
		name := strings.TrimSuffix(p, ".so") + ".so"

		if name == ".so" {
			continue // Just ignore it. uhg.
		}

		if _, err := os.Stat(name); os.IsNotExist(err) {
			name = path.Join(DefaultObjPath, name)
		}

		u.Logf("Loading Dynamic Plugin: %s", name)

		if _, err := plugin.Open(name); err != nil {
			return err
		}
	}

	return nil
}

// ParseConfigs parses the poller config and the config for each registered output plugin.
func (u *UnifiPoller) ParseConfigs() error {
	// Parse core config.
	if err := u.parseInterface(u.Config); err != nil {
		return err
	}

	// Load dynamic plugins.
	if err := u.LoadPlugins(); err != nil {
		return err
	}

	if err := u.parseInputs(); err != nil {
		return err
	}

	return u.parseOutputs()
}

// getFirstFile returns the first file that exists and is "reachable"
func getFirstFile(files []string) (string, error) {
	var err error

	for _, f := range files {
		if _, err = os.Stat(f); err == nil {
			return f, nil
		}
	}

	return "", err
}

// parseInterface parses the config file and environment variables into the provided interface.
func (u *UnifiPoller) parseInterface(i interface{}) error {
	// Parse config file into provided interface.
	if err := cnfgfile.Unmarshal(i, u.Flags.ConfigFile); err != nil {
		return err
	}

	// Parse environment variables into provided interface.
	_, err := cnfg.UnmarshalENV(i, ENVConfigPrefix)

	return err
}

// Parse input plugin configs.
func (u *UnifiPoller) parseInputs() error {
	inputSync.Lock()
	defer inputSync.Unlock()

	for _, i := range inputs {
		if err := u.parseInterface(i.Config); err != nil {
			return err
		}
	}

	return nil
}

// Parse output plugin configs.
func (u *UnifiPoller) parseOutputs() error {
	outputSync.Lock()
	defer outputSync.Unlock()

	for _, o := range outputs {
		if err := u.parseInterface(o.Config); err != nil {
			return err
		}
	}

	return nil
}
