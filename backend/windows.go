// +build windows

package backend

/*
#include <windows.h>
#include <winioctl.h>
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"

	"github.com/StackExchange/wmi"
	"golang.org/x/sys/windows"
)

var RelativeRawVMDK = false
var SupportPassiveListener = false

type Win32_LogicalDisk struct {
	DriveType  uint32
	Caption    string
	VolumeName *string
	DeviceID   string
}

type Win32_DiskPartition struct {
	DeviceID string
}

type Win32_DiskDrive struct {
	DeviceID string
	Name     string
}

type DiskGeometry struct {
	Cylinders         uint64
	MediaType         uint32
	TracksPerCylinder uint32
	SectorsPerTrack   uint32
	BytesPerSector    uint32
}

type GetLengthInformation struct {
	Size uint64
}

func IsAdmin() bool {
	return true
}

func controlCode(t, f, m, a int) int {
	return (((t) << 16) | ((a) << 14) | ((f) << 2) | (m))
}

func GetDeviceSize(device string) (uint64, error) {
	fd, err := windows.Open(device, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer windows.Close(fd)

	var length C.GET_LENGTH_INFORMATION
	var bytesReturned uint32
	buffer := make([]byte, unsafe.Sizeof(length))
	err = windows.DeviceIoControl(fd, C.IOCTL_DISK_GET_LENGTH_INFO, nil, 0, &buffer[0], uint32(len(buffer)), &bytesReturned, nil)
	if err != nil {
		return 0, err
	}

	var size uint64
	if err := binary.Read(bytes.NewReader(buffer), binary.LittleEndian, &size); err != nil {
		return 0, err
	}

	return size, nil
}

func FindDeviceByUUID(uuid string) (string, error) {
	return "", DeviceNotFound
}

func getDiskFromPartition(partition string) (string, error) {
	var parts []Win32_DiskPartition
	query := fmt.Sprintf("ASSOCIATORS OF {Win32_LogicalDisk.DeviceID=\"%s\"} WHERE AssocClass = Win32_LogicalDiskToPartition", partition)
	err := wmi.Query(query, &parts)
	if err != nil {
		return "", err
	}

	if len(parts) > 0 {
		var drives []Win32_DiskDrive
		err = wmi.Query(fmt.Sprintf("ASSOCIATORS OF {Win32_DiskPartition.DeviceID=\"%s\"} WHERE ResultClass = Win32_DiskDrive", parts[0].DeviceID), &drives)
		if err != nil {
			return "", err
		}
		return drives[0].DeviceID, nil
	}

	return "", DeviceNotFound
}

func getUsbDevices() (devices []USBDevice, err error) {
	var logicalDisks []Win32_LogicalDisk
	q := wmi.CreateQuery(&logicalDisks, "WHERE DriveType = 2")
	err = wmi.Query(q, &logicalDisks)
	if err != nil {
		return devices, err
	}

	for _, logicalDisk := range logicalDisks {
		disk, err := getDiskFromPartition(logicalDisk.DeviceID)
		if err != nil {
			return devices, err
		}

		devices = append(devices, USBDevice{
			Mountpoint: logicalDisk.Caption + "\\",
			VolumeName: logicalDisk.Caption + "_" + *logicalDisk.VolumeName,
			Device:     disk,
		})
	}

	return devices, nil
}

func FindDeviceByPath(path string) (string, error) {
	usbDevices, err := getUsbDevices()
	if err != nil {
		return "", err
	}
	log.Printf("Found USB devices: %+v\n", usbDevices)

	for _, device := range usbDevices {
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(device.Mountpoint)) {
			log.Printf("Found device %s\n", device.Device)
			return device.Device, nil
		}
	}

	return "", DeviceNotFound
}

func RunAsRoot(executable string, args ...string) error {
	return errors.New("Failed to find a way to run as root")
}

type windowsDevice struct {
	fd windows.Handle
}

func (w *windowsDevice) Read(p []byte) (n int, err error) {
	return windows.Read(w.fd, p)
}

func (w *windowsDevice) Write(p []byte) (n int, err error) {
	return windows.Write(w.fd, p)
}

func (w *windowsDevice) Seek(offset int64, whence int) (int64, error) {
	return windows.Seek(w.fd, offset, whence)
}

func (w *windowsDevice) Close() error {
	return windows.Close(w.fd)
}

func OpenDevice(device string, mode int) (DeviceFile, error) {
	fd, err := windows.Open(device, mode, 0)
	if err != nil {
		return nil, err
	}

	dev := &windowsDevice{fd: fd}

	return dev, nil
}
