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
	"path"
	"strings"

	"github.com/bishopfox/sliver/server/log"
	"github.com/shirou/gopsutil/mem"
)

const (
	goDirName = "go"

	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
)

var (
	gogoLog = log.NamedLogger("gogo", "compiler")

	// ValidCompilerTargets - Supported compiler targets
	ValidCompilerTargets = map[string]bool{
		"aix/ppc64":       true,
		"android/386":     true,
		"android/amd64":   true,
		"android/arm":     true,
		"android/arm64":   true,
		"darwin/amd64":    true,
		"darwin/arm64":    true,
		"dragonfly/amd64": true,
		"freebsd/386":     true,
		"freebsd/amd64":   true,
		"freebsd/arm":     true,
		"freebsd/arm64":   true,
		"illumos/amd64":   true,
		"ios/amd64":       true,
		"ios/arm64":       true,
		"js/wasm":         true,
		"linux/386":       true,
		"linux/amd64":     true,
		"linux/arm":       true,
		"linux/arm64":     true,
		"linux/mips":      true,
		"linux/mips64":    true,
		"linux/mips64le":  true,
		"linux/mipsle":    true,
		"linux/ppc64":     true,
		"linux/ppc64le":   true,
		"linux/riscv64":   true,
		"linux/s390x":     true,
		"netbsd/386":      true,
		"netbsd/amd64":    true,
		"netbsd/arm":      true,
		"netbsd/arm64":    true,
		"openbsd/386":     true,
		"openbsd/amd64":   true,
		"openbsd/arm":     true,
		"openbsd/arm64":   true,
		"openbsd/mips64":  true,
		"plan9/386":       true,
		"plan9/amd64":     true,
		"plan9/arm":       true,
		"solaris/amd64":   true,
		"windows/386":     true,
		"windows/amd64":   true,
		"windows/arm":     true,
	}
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
	return path.Join(appDir, goDirName)
}

// GetGoCache - Get the OS temp dir (used for GOCACHE)
func GetGoCache(appDir string) string {
	cachePath := path.Join(GetGoRootDir(appDir), "cache")
	os.MkdirAll(cachePath, 0700)
	return cachePath
}

// GetGoModCache - Get the GoMod cache dir
func GetGoModCache(appDir string) string {
	cachePath := path.Join(GetGoRootDir(appDir), "modcache")
	os.MkdirAll(cachePath, 0700)
	return cachePath
}

// The 6Gb limit here is somewhat arbitrary but is based on my own testing
func garbleMaxLiteralSize() []string {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		gogoLog.Errorf("Failed to detect amount of system memory: %s", err)
		return []string{} // Use default
	}
	if 6*gb < vmStat.Total {
		gogoLog.Infof("More than 6Gb of system memory, enable large literal obfuscation")
		return []string{"-literals-max-size", fmt.Sprintf("%d", 512*kb)}
	}
	gogoLog.Infof("Less than 6Gb of system memory, disable large literal obfuscation")
	return []string{}
}

func seed() string {
	seed := make([]byte, 32)
	rand.Read(seed)
	return hex.EncodeToString(seed)
}

// GarbleCmd - Execute a go command
func GarbleCmd(config GoConfig, cwd string, command []string) ([]byte, error) {
	target := fmt.Sprintf("%s/%s", config.GOOS, config.GOARCH)
	if _, ok := ValidCompilerTargets[target]; !ok {
		return nil, fmt.Errorf(fmt.Sprintf("Invalid compiler target: %s", target))
	}
	garbleBinPath := path.Join(config.GOROOT, "bin", "garble")
	seed := fmt.Sprintf("-seed=%s", seed())
	garbleFlags := []string{"-literals", seed}
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
		fmt.Sprintf("PATH=%s:%s", path.Join(config.GOROOT, "bin"), os.Getenv("PATH")),
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
	goBinPath := path.Join(config.GOROOT, "bin", "go")
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
		fmt.Sprintf("PATH=%s:%s", path.Join(config.GOROOT, "bin"), os.Getenv("PATH")),
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
	if _, ok := ValidCompilerTargets[target]; !ok {
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
