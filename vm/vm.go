package vm

import (
	"fmt"
	"log"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/lebauce/vbox"
	"github.com/lebauce/vlaunch/backend"
	"github.com/lebauce/vlaunch/config"
	"github.com/lebauce/vlaunch/vmdk"
)

var controllerName = "IDE"

type EventHandler interface {
	OnGuestPropertyChanged(event vbox.GuestPropertyChangedEvent)
}

type VirtualMachine struct {
	machine       vbox.Machine
	console       vbox.Console
	controller    vbox.StorageController
	session       vbox.Session
	dd            vbox.Medium
	wg            sync.WaitGroup
	eventHandlers []EventHandler
}

func (vm *VirtualMachine) OnStateChanged(event vbox.Event) {
}

func (vm *VirtualMachine) RegisterEventHandler(handler EventHandler) {
	vm.eventHandlers = append(vm.eventHandlers, handler)
}

func (vm *VirtualMachine) Run() error {
	eventSource, err := vm.console.GetEventSource()
	if err != nil {
		return err
	}
	defer eventSource.Release()

	listener, err := eventSource.CreateListener()
	if err != nil {
		return err
	}
	defer listener.Release()

	interestingEvents := []uint32{
		vbox.EventType_OnMachineStateChanged,
		vbox.EventType_OnStateChanged,
		vbox.EventType_MachineEvent,
		vbox.EventType_OnSessionStateChanged,
		vbox.EventType_OnGuestPropertyChanged,
	}
	if err := eventSource.RegisterListener(listener, interestingEvents, false); err != nil {
		return err
	}
	defer eventSource.UnregisterListener(listener)

	var wg sync.WaitGroup
	mainLoop := func() error {
		defer wg.Done()

		for {
			event, err := eventSource.GetEvent(listener, 250)
			if err != nil {
				return err
			}

			if event == nil {
				continue
			}

			eventType, err := event.GetType()
			if err != nil {
				return err
			}

			state, err := vm.machine.GetState()
			if err != nil {
				return err
			}

			switch eventType {
			case vbox.EventType_OnStateChanged:
				vm.OnStateChanged(*event)
			case vbox.EventType_OnGuestPropertyChanged:
				guestPropEvent, err := vbox.NewGuestPropertyChangedEvent(event)
				if err != nil {
					return err
				}
				for _, handler := range vm.eventHandlers {
					handler.OnGuestPropertyChanged(guestPropEvent)
				}
			default:
			}

			if eventType == vbox.EventType_OnStateChanged && state == vbox.MachineState_PoweredOff {
				return nil
			}

			err = eventSource.EventProcessed(listener, *event)
			if err != nil {
				return err
			}

			event.Release()
		}
	}

	wg.Add(1)
	go func() {
		err = mainLoop()
	}()

	wg.Wait()
	return err
}

func (vm *VirtualMachine) Start() error {
	progress, err := vm.machine.Launch(vm.session, "gui", "")
	if err != nil {
		return err
	}

	if err = progress.WaitForCompletion(50000); err != nil {
		return err
	}
	progress.Release()

	console, err := vm.session.GetConsole()
	if err != nil {
		return err
	}

	vm.console = console
	return nil
}

func (vm *VirtualMachine) Stop() error {
	return nil
}

func (vm *VirtualMachine) Release() error {
	if err := vm.session.UnlockMachine(); err != nil {
		return err
	}
	time.Sleep(time.Second)

	if err := vm.controller.Release(); err != nil {
		return err
	}

	media, err := vm.machine.Unregister(vbox.CleanupMode_Full)
	if err != nil {
		return err
	}

	progress, err := vm.machine.DeleteConfig(media)
	if err != nil {
		return err
	}
	defer progress.Release()

	if err = progress.WaitForCompletion(-1); err != nil {
		return err
	}

	if err := vm.machine.Release(); err != nil {
		return err
	}

	/*
		if err := vm.session.Release(); err != nil {
			return err
		}
	*/

	return nil
}

func (vm *VirtualMachine) Create() error {
	cfg := config.GetConfig()
	settingsPath := path.Join(cfg.GetString("data_path"))

	if err := vbox.Init(); err != nil {
		return nil, fmt.Errorf("Failed to initialize VirtualBox API: %s", err.Error())
	}

	diskLocation := ""
	diskType := cfg.GetString("disk_type")
	switch diskType {
	case "raw":
		device, err := backend.FindDevice()
		if err != nil {
			return err
		}

		log.Printf("Creating raw VMDK for device %s\n", device)
		diskLocation = path.Join(settingsPath, "raw.vmdk")
		if err := vmdk.CreateRawVMDK(diskLocation, device, true, backend.RelativeRawVMDK); err != nil {
			return err
		}
	case "vdi":
		diskLocation = cfg.GetString("disk_location")
	default:
		return fmt.Errorf("Invalid disk type '%s'", diskType)
	}

	dd, err := vbox.OpenMedium(diskLocation, vbox.DeviceType_HardDisk,
		vbox.AccessMode_ReadWrite, false)
	if err != nil {
		return err
	}

	machine, err := vbox.CreateMachine(settingsPath, "ufo", cfg.GetString("distro_type"), "")
	if err != nil {
		return err
	}

	cpus := cfg.GetInt("cpus")
	if cpus <= 0 {
		if cpus = runtime.NumCPU(); cpus > 1 {
			cpus /= 2
		}
	}
	machine.SetCPUCount(uint(cpus))

	ram := cfg.GetInt("ram")
	if ram <= 0 {
		if freeRam, err := backend.GetFreeRam(); err == nil {
			ram = (int(freeRam) * 2 / 3) / 1024 / 1024
		}

		if minRam := cfg.GetInt("min_ram"); ram < minRam {
			ram = minRam
		}
	}
	log.Printf("Setting RAM to %d\n", ram)
	machine.SetMemorySize(uint(ram))

	if err := machine.SetVramSize(32); err != nil {
		return err
	}

	biosSettings, err := machine.GetBiosSettings()
	if err != nil {
		return err
	}

	biosSettings.SetACPIEnabled(true)
	biosSettings.SetIOAPICEnabled(true)
	biosSettings.SetBootMenuMode(vbox.BootMenuMode_Disabled)

	adapter, err := machine.GetNetworkAdapter(0)
	if err != nil {
		return err
	}

	if err := adapter.SetAdapterType(vbox.NetworkAdapterType_I82540EM); err != nil {
		return err
	}

	// TODO: set audio adapter

	machine.SetExtraData("GUI/MaxGuestResolution", "any")

	if hostKey := cfg.GetString("host_key"); hostKey != "" {
		machine.SetExtraData("GUI/Input/HostKey", hostKey)
	}

	machine.SetAccelerate3DEnabled(true)
	machine.SetDnDMode(vbox.DnDMode_Bidirectional)
	machine.SetClipboardMode(vbox.ClipboardMode_Bidirectional)

	for name := range cfg.GetStringMap("shared_folders") {
		sharedFolder := cfg.Sub("shared_folders." + name)
		path := sharedFolder.GetString("path")
		persistent := sharedFolder.GetBool("persistent")
		automount := sharedFolder.GetBool("automount")
		if err := machine.CreateSharedFolder(name, path, persistent, automount); err != nil {
			log.Printf("Failed to create shared folder %s: %s", name, err.Error())
		}
	}

	controller, err := machine.AddStorageController(controllerName, vbox.StorageBus_Ide)
	if err != nil {
		return err
	}

	if err = controller.SetType(vbox.StorageControllerType_Ich6); err != nil {
		return err
	}

	if err := machine.SaveSettings(); err != nil {
		return err
	}

	if err := machine.Register(); err != nil {
		return err
	}

	session := vbox.Session{}
	if err := session.Init(); err != nil {
		return err
	}

	if err := session.LockMachine(machine, vbox.LockType_Write); err != nil {
		return err
	}

	// NOTE: Machine modifications require the mutable instance obtained from
	smachine, err := session.GetMachine()
	if err != nil {
		return err
	}

	if err := smachine.AttachDevice(controllerName, 0, 0, vbox.DeviceType_HardDisk, dd); err != nil {
		return err
	}

	if err = smachine.SaveSettings(); err != nil {
		return err
	}

	if err := session.UnlockMachine(); err != nil {
		return err
	}

	vm.machine = machine
	vm.controller = controller
	vm.session = session
	vm.dd = dd

	return nil
}

func NewVM() (*VirtualMachine, error) {
	return &VirtualMachine{}, nil
}
