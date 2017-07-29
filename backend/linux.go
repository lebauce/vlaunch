// +build linux

package backend

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/guillermo/go.procmeminfo"
)

var RelativeRawVMDK = true

func OpenDevice(device string, mode int) (DeviceFile, error) {
	return os.OpenFile(device, mode, 0)
}

func GetFreeRam() (uint64, error) {
	meminfo := &procmeminfo.MemInfo{}
	if err := meminfo.Update(); err != nil {
		return 0, err
	}
	return meminfo.Available(), nil
}

func GetDeviceSize(device string) (uint64, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/sys/block/%s/size", path.Base(device)))
	if err != nil {
		return 0, err
	}

	size, err := strconv.Atoi(strings.TrimSpace(string(content)))
	return uint64(size), err
}

func FindDeviceByUUID(uuid string) (string, error) {
	matches, err := filepath.Glob("/dev/sd?[0-9]")
	if err != nil {
		return "", err
	}

	for _, device := range matches {
		output, _ := exec.Command("/usr/sbin/blkid", "-o", "value", "-s", "UUID", "-c", "/dev/null", device).Output()
		if strings.TrimSpace(string(output)) == uuid {
			return strings.TrimRightFunc(device, unicode.IsDigit), nil
		}
	}

	return "", DeviceNotFound
}

func FindDeviceByPath(path string) (string, error) {
	output, _ := exec.Command("/usr/bin/findmnt", "-v", "-n", "-o", "SOURCE", "--target", path).Output()
	if device := strings.TrimSpace(string(output)); device != "" {
		return strings.TrimRightFunc(device, unicode.IsDigit), nil
	}

	return "", DeviceNotFound
}

func RunAsRoot(executable string, args ...string) error {
	if _, err := os.Stat("/usr/bin/beesu"); err == nil {
		rootArgs := []string{}
		for _, environ := range os.Environ() {
			if strings.HasPrefix(environ, "VLAUNCH_") || strings.HasPrefix(environ, "VBOX_") {
				rootArgs = append(rootArgs, environ)
			}
		}
		rootArgs = append(rootArgs, executable)
		rootArgs = append(rootArgs, args...)
		log.Printf("Running /usr/bin/beesu %s", strings.Join(rootArgs, " "))
		cmd := exec.Command("/usr/bin/beesu", strings.Join(rootArgs, " "))
		return cmd.Start()
	}

	return errors.New("Failed to find a way to run as root")
}

func IsAdmin() bool {
	return os.Geteuid() == 0
}
