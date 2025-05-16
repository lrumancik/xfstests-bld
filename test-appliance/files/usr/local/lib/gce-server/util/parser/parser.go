/*
Package parser parses a gce-xfstests command line to distribute tests among
multiple shards.
*/
package parser

import (
	"encoding/base64"
	"fmt"
	"strings"
	"thunk.org/gce-server/util/check"
)

const (
	primaryFS = "ext4"
	xfsPath   = "/root"
)

var invalidBools = []string{
	"ltm",
	"--no-region-shard",
	"--no-email",
	"--no-junit-email",
}
var invalidOpts = []string{
	"--instance-name",
	"--bucket-subdir",
	"--gs-bucket",
	"--email",
	"--fail-email",
	"--junit-email",
	"--gce-zone",
	"--testrunid",
	"--hooks",
	"--update-xfstests-tar",
	"--update-xfstests",
	"--update-files",
	"-n",
	"-r",
	"--machtype",
	"--kernel",
	"--arch",
	"--commit",
	"--config",
	"--kconfig-opts",
	"--repo",
	"--watch",
	"--bisect-good",
	"--bisect-bad",
	"--monitor-timeout",
}

/*
Cmd parses a cmdline into validArgs and configs.

Returns:

	validArgs - a slice of cmd args not related to test configurations.
	Parser removes arguments from the original cmd that don't make sense
	for LTM (e.g. ltm, --instance-name).

	configs - a map from filesystem names to a slice of corresponding
	configurations.  Duplicates are removed from the original cmd configs.
*/
func Cmd(cmdLine string) ([]string, map[string][]string, error) {
	args := strings.Fields(cmdLine)
	validArgs, _ := sanitizeCmd(args)
	validArgs = expandAliases(validArgs)
	return processConfigs(validArgs)
}

// sanitizeCmd removes invalid args from input cmdline.
func sanitizeCmd(args []string) ([]string, []string) {
	boolDict := NewSet(invalidBools)
	optDict := NewSet(invalidOpts)
	validArgs := []string{}
	invalidArgs := []string{}
	skipIndex := false

	for _, arg := range args {
		if skipIndex {
			invalidArgs = append(invalidArgs, arg)
			skipIndex = false
		} else {
			if boolDict.Contain(arg) {
				invalidArgs = append(invalidArgs, arg)
			} else if optDict.Contain(arg) {
				invalidArgs = append(invalidArgs, arg)
				skipIndex = true
			} else {
				validArgs = append(validArgs, arg)
			}
		}
	}
	return validArgs, invalidArgs
}

// expandAliases expands some explicit aliases of test options.
// It only converts "smoke" to "-c 4k -g quick", since other aliases
// ("full", "quick") have no effects on -c configs.
func expandAliases(args []string) []string {
	prefixArgs := []string{}
	expandedArgs := []string{}

	for _, arg := range args {
		if arg == "smoke" {
			if len(prefixArgs) == 0 {
				prefixArgs = append(prefixArgs, "-c", "4k", "-g", "quick")
			}
		} else {
			expandedArgs = append(expandedArgs, arg)
		}
	}

	expandedArgs = append(prefixArgs, expandedArgs...)
	return expandedArgs
}

// processConfigs finds the configuration args following "-c" and parses
// them. If no "-c" option is specified (or aliases like "smoke"), it uses
// primaryFS as the filesystem and "all" as the config.
func processConfigs(args []string) ([]string, map[string][]string, error) {
	newArgs := make([]string, len(args))
	copy(newArgs, args)
	configArg := ""
	configs := make(map[string][]string)

	for i, arg := range args {
		if arg == "-c" {
			configArg = args[i+1]
			newArgs = append(args[:i], args[i+2:]...)
			break
		}
	}

	if configArg == "" {
		err := defaultConfigs(configs)
		if err != nil {
			return newArgs, configs, err
		}
	} else {
		for _, c := range strings.Split(configArg, ",") {
			err := singleConfig(configs, c)
			if err != nil {
				return newArgs, configs, err
			}
		}
	}

	return newArgs, configs, nil
}

func defaultConfigs(configs map[string][]string) error {
	configFile := fmt.Sprintf("%s/fs/%s/cfg/all.list", xfsPath, primaryFS)
	lines, err := check.ReadLines(configFile)
	if err != nil {
		return err
	}

	for _, line := range lines {
		configs[primaryFS] = append(configs[primaryFS], line)
	}
	return nil
}

/*
singleConfig parses a single configuration and adds it to the map.
Possible pattern of configs:

	<fs>/<cfg> (e.g. ext4/4k) - checks /root/fs/<fs>/cfg/<cfg>.list
	for a list of configurations, and read config lines from each file.

	<fs> (e.g. ext4) - uses default config for <fs> if it exists.

	<cfg> (e.g. quick) - uses primaryFS and <cfg> as the configuration.

	<primaryFS>:<fs>/<cfg> (e.g. ext4:overlay/small) - runs the <fs>/<cfg>
	config with <primaryFS> set as primary file system
*/
func singleConfig(configs map[string][]string, configArg string) error {
	configLines := []string{}
	var fs, cfg, localPrimaryFS string

	arg := strings.Split(configArg, "/")
	if len(arg) == 1 {
		if check.FileExists(fmt.Sprintf("%s/fs/%s", xfsPath, configArg)) {
			fs = configArg
			configLines = []string{"default"}
		} else {
			fs = primaryFS
			cfg = configArg
		}
	} else {
		checkPrimary := strings.Split(arg[0], ":")
		if len(checkPrimary) > 1 {
			localPrimaryFS = checkPrimary[0]
			fs = checkPrimary[1]
		} else {
			fs = arg[0]
		}
		cfg = arg[1]
	}

	if len(configLines) == 0 {
		configFile := fmt.Sprintf("%s/fs/%s/cfg/%s.list", xfsPath, fs, cfg)

		if check.FileExists(configFile) {
			lines, err := check.ReadLines(configFile)
			if err != nil {
				return err
			}
			configLines = lines
		} else {
			configFile = configFile[:len(configFile)-5]

			if check.FileExists(configFile) {
				configLines = []string{cfg}
			} else {
				return nil
			}
		}
	}

	if len(localPrimaryFS) > 0 {
		fs = fmt.Sprintf("%s:%s", localPrimaryFS, fs)
	}

	if _, ok := configs[fs]; ok {
		configs[fs] = append(configs[fs], configLines...)
	} else {
		configs[fs] = configLines
	}
	return nil
}

// DecodeCmd decodes the base64 string in user requests.
func DecodeCmd(cmdLine string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cmdLine)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
