package vmdk

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"github.com/lebauce/vlaunch/backend"
	"github.com/rekby/gpt"
)

var blockSize uint64 = 512
var headerTemplate = `# Disk DescriptorFile
version=1
CID=8902101c
parentCID=ffffffff
createType="{{.Type}}"
{{range .Extents}}{{.AccessMode}} {{.Size}} {{.Type}}{{if .Path}} "{{.Path}}"{{end}}{{if eq .Type "FLAT"}} {{.Offset}}{{end}}
{{end}}ddb.virtualHWVersion = "4"
ddb.adapterType="ide"
ddb.geometry.cylinders="{{.Cylinders}}"
ddb.geometry.heads="16"
ddb.geometry.sectors="63"
ddb.geometry.biosCylinders="{{.Cylinders}}"
ddb.geometry.biosHeads="16"
ddb.geometry.biosSectors="63"
ddb.uuid.image="{{.UUID}}"
ddb.uuid.parent="00000000-0000-0000-0000-000000000000"
ddb.uuid.modification="b0004a36-2323-433e-9bbc-103368bc5e41"
ddb.uuid.parentmodification="00000000-0000-0000-0000-000000000000"`

type rawVMDK struct {
	UUID       uuid.UUID
	TargetPath string
	DeviceName string
	DeviceSize uint64
	Type       string
	Cylinders  uint64
	Extents    []extent
}

type extent struct {
	AccessMode string
	Size       uint64
	Type       string
	Path       string
	Offset     uint64
}

func CreateRawVMDK(location string, deviceName string, partitions bool, relative bool) error {
	deviceSize, err := backend.GetDeviceSize(deviceName)
	if err != nil {
		return err
	}

	cylinders := deviceSize / 16 / 64
	if cylinders > 16383 {
		cylinders = 16383
	}

	vmdk := rawVMDK{
		UUID:       uuid.New(),
		DeviceName: deviceName,
		DeviceSize: deviceSize,
		Cylinders:  cylinders,
	}

	if partitions {
		dev, err := os.Open(deviceName)
		if err != nil {
			panic(fmt.Sprintf("Failed to open device: %s", err.Error()))
		}
		defer dev.Close()

		if _, err := dev.Seek(512, io.SeekStart); err != nil {
			return fmt.Errorf("Failed to seek to GPT: %s", err.Error())
		}

		table, err := gpt.ReadTable(dev, blockSize)
		if err != nil {
			panic(fmt.Sprintf("Failed to read GPT table: %s", err.Error()))
		}

		headerPath := strings.TrimSuffix(location, path.Ext(location)) + "-pt.vmdk"
		deviceHeader, err := os.Create(headerPath)
		if err != nil {
			return err
		}
		defer deviceHeader.Close()

		if _, err := dev.Seek(0, io.SeekStart); err != nil {
			return err
		}

		offset := table.Partitions[0].FirstLBA
		if _, err := io.CopyN(deviceHeader, dev, int64(offset*blockSize)); err != nil {
			return err
		}

		header := extent{AccessMode: "RW", Size: offset, Type: "FLAT", Path: headerPath}
		vmdk.Type = "partitionedDevice"
		vmdk.Extents = append(vmdk.Extents, header)

		for i, part := range table.Partitions {
			if part.IsEmpty() {
				break
			}

			if part.FirstLBA > offset {
				vmdk.Extents = append(vmdk.Extents, extent{
					AccessMode: "RW",
					Size:       part.FirstLBA - offset,
					Type:       "ZERO",
				})
			}

			size := part.LastLBA - part.FirstLBA + 1
			newExtent := extent{
				AccessMode: "RW",
				Size:       size,
				Type:       "FLAT",
				Offset:     part.FirstLBA,
				Path:       deviceName,
			}

			if relative {
				newExtent.Path = fmt.Sprintf("%s%d", deviceName, i+1)
				newExtent.Offset = 0
			}

			vmdk.Extents = append(vmdk.Extents, newExtent)
			offset += size
		}

		vmdk.Extents = append(vmdk.Extents, extent{
			AccessMode: "RW",
			Size:       deviceSize - offset,
			Type:       "ZERO",
		})
	} else {
		vmdk.Type = "fullDevice"
		vmdk.Extents = []extent{
			extent{AccessMode: "RW", Size: deviceSize, Type: "FLAT", Path: deviceName},
		}
	}

	t := template.Must(template.New("VMDK").Parse(headerTemplate))

	file, err := os.Create(location)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := t.Execute(file, vmdk); err != nil {
		return err
	}

	return nil
}
