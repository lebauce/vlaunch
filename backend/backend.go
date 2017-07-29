package backend

import (
	"errors"
	"io"
	"os"

	"github.com/lebauce/vlaunch/config"
)

var DeviceNotFound = errors.New("Could not find device")

type USBDevice struct {
	Mountpoint string
	VolumeName string
	Device     string
}

type DeviceFile interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
}

func FindDevice() (string, error) {
	if device := config.GetConfig().GetString("device"); device != "" {
		return device, nil
	}

	if uuid := config.GetConfig().GetString("device_uuid"); uuid != "" {
		if device, err := FindDeviceByUUID(uuid); err == nil {
			return device, nil
		}
	}

	if executable, err := os.Executable(); err == nil {
		if device, err := FindDeviceByPath(executable); err == nil {
			return device, nil
		}
	}

	return "", DeviceNotFound
}
