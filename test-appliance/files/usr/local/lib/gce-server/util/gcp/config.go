package gcp

import (
	"fmt"
	"regexp"
	"sync"

	"thunk.org/gce-server/util/check"
)

// config file locations on multiple machines
const gce = "/usr/local/lib/gce_xfstests.config"

// Config stores the parsed config key value pairs.
type Config struct {
	kv map[string]string
}

// Global config structs initialized at boot time.
// GceConfig should always be not nil.
var (
	GceConfig  *Config
	LTMConfig  *Config
	KCSConfig  *Config
	configLock sync.RWMutex
	ltm        string
	kcs        string
)

func init() {
	configLock.Lock()
	defer configLock.Unlock()

	var err error
	GceConfig, err = Get(gce)
	if err != nil {
		panic("failed to parse gce config file")
	}

	// get GCE_PROJECT directly, do not go through Get function
	// as init is already holding the configLock
	var projID = GceConfig.kv["GCE_PROJECT"]
	if projID == "" {
		panic("failed to get GCE_PROJECT from gce config")
	}

	ltm = "/root/xfstests_bld/run-fstests/.ltm_instance_" + projID
	kcs = "/root/xfstests_bld/run-fstests/.kcs_instance_" + projID

	if check.FileExists(ltm) {
		LTMConfig, err = Get(ltm)
		if err != nil {
			panic("failed to parse LTM config file")
		}
	}

	if check.FileExists(kcs) {
		KCSConfig, err = Get(kcs)
		if err != nil {
			panic("failed to parse KCS config file")
		}
	}
}

// Update reads three config files to generate new config kv pairs.
// It should be called after executing launch-ltm/launch-kcs.
func Update() error {
	configLock.Lock()
	defer configLock.Unlock()

	var err error
	GceConfig, err = Get(gce)
	if err != nil {
		return fmt.Errorf("failed to parse gce config file")
	}

	if check.FileExists(ltm) {
		LTMConfig, err = Get(ltm)
		if err != nil {
			return fmt.Errorf("failed to parse LTM config file")
		}
	}

	if check.FileExists(kcs) {
		KCSConfig, err = Get(kcs)
		if err != nil {
			return fmt.Errorf("failed to parse KCS config file")
		}
	}

	return nil
}

// Get reads from the config file and returns a struct Config.
// It attempts to match each line with two possible config patterns.
func Get(configFile string) (*Config, error) {
	c := Config{make(map[string]string)}
	re := regexp.MustCompile(`(?:(^declare (?:--|-x) (?P<key>\S+)="(?P<value>\S*)"$)|(^(?P<key>\S+)=(?P<value>\S*))$)`)

	lines, err := check.ReadLines(configFile)
	if err != nil {
		return &c, err
	}

	for _, line := range lines {
		tokens := re.FindStringSubmatch(line)
		if len(tokens) == 0 {
			continue
		}
		var key, value string
		for i, name := range re.SubexpNames() {
			if name == "key" && tokens[i] != "" {
				key = tokens[i]
			}
			if name == "value" && tokens[i] != "" {
				value = tokens[i]
			}
		}

		if key != "" {
			c.kv[key] = value
		}
	}

	return &c, nil
}

// Get a certain config value according to key.
// Return empty value of error if key is not present in config.
func (c *Config) Get(key string) (string, error) {
	configLock.RLock()
	defer configLock.RUnlock()
	if c == nil {
		return "", fmt.Errorf("config is nil")
	}
	if val, ok := c.kv[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("%s not found in config file", key)
}
