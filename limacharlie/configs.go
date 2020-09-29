package limacharlie

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"strings"

	"gopkg.in/yaml.v2"
)

// The actual config file format may seem a bit odd
// but it is structured to maintain backwards compatibility
// with the Python SDK/CLI format.
type ConfigFile struct {
	ConfigEnvironment
	Environments map[string]ConfigEnvironment `yaml:"env"`
}

type ConfigEnvironment struct {
	OID    string `yaml:"oid"`
	UID    string `yaml:"uid"`
	APIKey string `yaml:"api_key"`
}

func (o *ClientOptions) FromConfigFile(configFilePath string, environmentName string) error {
	cleanPath := configFilePath
	if strings.HasPrefix(cleanPath, "~/") {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		dir := usr.HomeDir
		cleanPath = fmt.Sprintf("%s/%s", dir, cleanPath[2:])
	}
	data, err := ioutil.ReadFile(cleanPath)
	if err != nil {
		return err
	}
	return o.FromConfigString(data, environmentName)
}

func (o *ClientOptions) FromConfigString(configFileString []byte, environmentName string) error {
	cfg := ConfigFile{}
	if err := yaml.Unmarshal(configFileString, &cfg); err != nil {
		return err
	}
	if err := yaml.Unmarshal(configFileString, &cfg.ConfigEnvironment); err != nil {
		return err
	}
	return o.FromConfig(cfg, environmentName)
}

func (o *ClientOptions) FromConfig(cfg ConfigFile, environmentName string) error {
	// An empty environment name defaults.
	if environmentName == "" {
		environmentName = "default"
	}

	// Load the relevant environment.
	var env ConfigEnvironment
	var ok bool
	if environmentName == "default" {
		env = cfg.ConfigEnvironment
	} else if env, ok = cfg.Environments[environmentName]; !ok {
		return NewInvalidClientOptionsError(fmt.Sprintf("environment %s not found", environmentName))
	}

	// Set the values, validation is done by the client itself.
	o.OID = env.OID
	o.UID = env.UID
	o.APIKey = env.APIKey

	return nil
}
