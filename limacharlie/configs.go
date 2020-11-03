package limacharlie

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os/user"
	"strings"
)

// ConfigFile is the actual config file format may seem a bit odd
// but it is structured to maintain backwards compatibility
// with the Python SDK/CLI format.
type ConfigFile struct {
	ConfigEnvironment
	Environments map[string]ConfigEnvironment `yaml:"env"`
}

// ConfigEnvironment holds the different values parsed from the environment
type ConfigEnvironment struct {
	OID    string `yaml:"oid"`
	UID    string `yaml:"uid"`
	APIKey string `yaml:"api_key"`
}

// FromConfigFile updates self from the file path
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

// FromConfigString updates self from strings
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

// FromConfig updates self from a config file
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
