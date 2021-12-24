package gogo

/*
	Sliver Implant Framework
	Copyright (C) 2019  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bishopfox/sliver/server/log"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	goDirName = "go"

	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
)

var (
	gogoLog = log.NamedLogger("gogo", "compiler")
)

// GoConfig - Env variables for Go compiler
type GoConfig struct {
	ProjectDir string

	GOOS       string
	GOARCH     string
	GOROOT     string
	GOCACHE    string
	GOMODCACHE string
	GOPROXY    string
	CGO        string
	CC         string
	CXX        string

	Obfuscation bool
	GOPRIVATE   string
}

// GetGoRootDir - Get the path to GOROOT
func GetGoRootDir(appDir string) string {
	return filepath.Join(appDir, goDirName)
}

// GetGoCache - Get the OS temp dir (used for GOCACHE)
func GetGoCache(appDir string) string {
	cachePath := filepath.Join(GetGoRootDir(appDir), "cache")
	os.MkdirAll(cachePath, 0700)
	return cachePath
}

// GetGoModCache - Get the GoMod cache dir
func GetGoModCache(appDir string) string {
	cachePath := filepath.Join(GetGoRootDir(appDir), "modcache")
	os.MkdirAll(cachePath, 0700)
	return cachePath
}

// The Gb limit here is somewhat arbitrary but is based on my own testing
func garbleMaxLiteralSize() []string {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		gogoLog.Errorf("Failed to detect amount of system memory: %s", err)
		return []string{"-literals-max-size", fmt.Sprintf("%d", 1*kb)} // Use default
	}
	if 10*gb < vmStat.Total {
		gogoLog.Infof("More than 10Gb of system memory, enable large literal obfuscation")
		return []string{"-literals-max-size", fmt.Sprintf("%d", 64*kb)}
	}
	gogoLog.Infof("Low system memory, disable large literal obfuscation")
	return []string{"-literals-max-size", fmt.Sprintf("%d", 2*kb)}
}

func seed() string {
	seed := make([]byte, 32)
	rand.Read(seed)
	return hex.EncodeToString(seed)
}

// GarbleCmd - Execute a go command
func GarbleCmd(config GoConfig, cwd string, command []string) ([]byte, error) {
	target := fmt.Sprintf("%s/%s", config.GOOS, config.GOARCH)
	if _, ok := ValidCompilerTargets(config)[target]; !ok {
		return nil, fmt.Errorf(fmt.Sprintf("Invalid compiler target: %s", target))
	}
	garbleBinPath := filepath.Join(config.GOROOT, "bin", "garble")
	garbleFlags := []string{fmt.Sprintf("-seed=%s", seed()), "-literals"}
	garbleFlags = append(garbleFlags, garbleMaxLiteralSize()...)
	command = append(garbleFlags, command...)
	cmd := exec.Command(garbleBinPath, command...)
	cmd.Dir = cwd
	cmd.Env = []string{
		fmt.Sprintf("CC=%s", config.CC),
		fmt.Sprintf("CGO_ENABLED=%s", config.CGO),
		fmt.Sprintf("GOOS=%s", config.GOOS),
		fmt.Sprintf("GOARCH=%s", config.GOARCH),
		fmt.Sprintf("GOPATH=%s", config.ProjectDir),
		fmt.Sprintf("GOCACHE=%s", config.GOCACHE),
		fmt.Sprintf("GOMODCACHE=%s", config.GOMODCACHE),
		fmt.Sprintf("GOPRIVATE=%s", config.GOPRIVATE),
		fmt.Sprintf("GOPROXY=%s", config.GOPROXY),
		fmt.Sprintf("PATH=%s:%s", filepath.Join(config.GOROOT, "bin"), os.Getenv("PATH")),
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	gogoLog.Infof("garble cmd: '%v'", cmd)
	err := cmd.Run()
	if err != nil {
		gogoLog.Infof("--- env ---\n")
		for _, envVar := range cmd.Env {
			gogoLog.Infof("%s\n", envVar)
		}
		gogoLog.Infof("--- stdout ---\n%s\n", stdout.String())
		gogoLog.Infof("--- stderr ---\n%s\n", stderr.String())
		gogoLog.Info(err)
	}

	return stdout.Bytes(), err
}

// GoCmd - Execute a go command
func GoCmd(config GoConfig, cwd string, command []string) ([]byte, error) {
	goBinPath := filepath.Join(config.GOROOT, "bin", "go")
	cmd := exec.Command(goBinPath, command...)
	cmd.Dir = cwd
	cmd.Env = []string{
		fmt.Sprintf("CC=%s", config.CC),
		fmt.Sprintf("CGO_ENABLED=%s", config.CGO),
		fmt.Sprintf("GOOS=%s", config.GOOS),
		fmt.Sprintf("GOARCH=%s", config.GOARCH),
		fmt.Sprintf("GOPATH=%s", config.ProjectDir),
		fmt.Sprintf("GOCACHE=%s", config.GOCACHE),
		fmt.Sprintf("GOMODCACHE=%s", config.GOMODCACHE),
		fmt.Sprintf("GOPROXY=%s", config.GOPROXY),
		fmt.Sprintf("PATH=%s:%s", filepath.Join(config.GOROOT, "bin"), os.Getenv("PATH")),
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	gogoLog.Infof("go cmd: '%v'", cmd)
	err := cmd.Run()
	if err != nil {
		gogoLog.Infof("--- env ---\n")
		for _, envVar := range cmd.Env {
			gogoLog.Infof("%s\n", envVar)
		}
		gogoLog.Infof("--- stdout ---\n%s\n", stdout.String())
		gogoLog.Infof("--- stderr ---\n%s\n", stderr.String())
		gogoLog.Info(err)
	}

	return stdout.Bytes(), err
}

// GoBuild - Execute a go build command, returns stdout/error
func GoBuild(config GoConfig, src string, dest string, buildmode string, tags []string, ldflags []string, gcflags, asmflags string, trimpath string) ([]byte, error) {
	target := fmt.Sprintf("%s/%s", config.GOOS, config.GOARCH)
	if _, ok := ValidCompilerTargets(config)[target]; !ok {
		return nil, fmt.Errorf(fmt.Sprintf("Invalid compiler target: %s", target))
	}
	var goCommand = []string{"build"}
	if 0 < len(trimpath) {
		goCommand = append(goCommand, trimpath)
	}
	if 0 < len(tags) {
		goCommand = append(goCommand, "-tags")
		goCommand = append(goCommand, tags...)
	}
	if 0 < len(ldflags) {
		goCommand = append(goCommand, "-ldflags")
		goCommand = append(goCommand, ldflags...)
	}
	if 0 < len(gcflags) {
		goCommand = append(goCommand, fmt.Sprintf("-gcflags=%s", gcflags))
	}
	if 0 < len(asmflags) {
		goCommand = append(goCommand, fmt.Sprintf("-asmflags=%s", asmflags))
	}
	if 0 < len(buildmode) {
		goCommand = append(goCommand, fmt.Sprintf("-buildmode=%s", buildmode))
	}
	goCommand = append(goCommand, []string{"-o", dest, "."}...)
	if config.Obfuscation {
		return GarbleCmd(config, src, goCommand)
	}
	return GoCmd(config, src, goCommand)
}

// GoMod - Execute go module commands in src dir
func GoMod(config GoConfig, src string, args []string) ([]byte, error) {
	goCommand := []string{"mod"}
	goCommand = append(goCommand, args...)
	return GoCmd(config, src, goCommand)
}

// GoVersion - Execute a go version command, returns stdout/error
func GoVersion(config GoConfig) ([]byte, error) {
	var goCommand = []string{"version"}
	wd, _ := os.Getwd()
	return GoCmd(config, wd, goCommand)
}

// ValidCompilerTargets - Returns a map of valid compiler targets
func ValidCompilerTargets(config GoConfig) map[string]bool {
	validTargets := make(map[string]bool)
	for _, target := range GoToolDistList(config) {
		validTargets[target] = true
	}
	return validTargets
}

// GoToolDistList - Get a list of supported GOOS/GOARCH pairs
func GoToolDistList(config GoConfig) []string {
	var goCommand = []string{"tool", "dist", "list"}
	wd, _ := os.Getwd()
	data, err := GoCmd(config, wd, goCommand)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	return lines
}
