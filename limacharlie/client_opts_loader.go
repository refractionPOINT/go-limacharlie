package limacharlie

import (
	"os"
)

// ClientOptionLoader loads options for the limacharlie client
type ClientOptionLoader interface {
	Load(inOpt ClientOptions) (ClientOptions, error)
}

// NoopClientOptionLoader does not load any options
type NoopClientOptionLoader struct{}

// Load returns arguments passed
func (l *NoopClientOptionLoader) Load(inOpt ClientOptions) (ClientOptions, error) {
	return inOpt, nil
}

// EnvironmentClientOptionLoader loads options from environement variables
type EnvironmentClientOptionLoader struct{}

// Load retrieves options from environment variables
func (l *EnvironmentClientOptionLoader) Load(inOpt ClientOptions) (ClientOptions, error) {
	opt := inOpt
	if isEmpty(opt.Environment) {
		opt.Environment = os.Getenv("LC_CURRENT_ENV")
	}
	if isEmpty(opt.OID) {
		opt.OID = os.Getenv("LC_OID")
	}
	if isEmpty(opt.UID) {
		opt.UID = os.Getenv("LC_UID")
	}
	if isEmpty(opt.APIKey) {
		opt.APIKey = os.Getenv("LC_API_KEY")
	}
	return opt, nil
}

// FileClientOptionLoader loads options from environement variables
type FileClientOptionLoader struct {
	path string
}

// NewFileClientOptionLoader initialize a new loader
func NewFileClientOptionLoader(configFile string) *FileClientOptionLoader {
	return &FileClientOptionLoader{
		path: configFile,
	}
}

// Load retrieve options from a config file
func (l *FileClientOptionLoader) Load(inOpt ClientOptions) (ClientOptions, error) {
	opts := ClientOptions{}
	if err := opts.FromConfigFile(l.path, inOpt.Environment); err != nil {
		return opts, err
	}
	return opts, nil
}
