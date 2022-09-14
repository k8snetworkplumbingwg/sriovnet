package sriovnet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/k8snetworkplumbingwg/sriovnet/pkg/utils/filesystem"
)

const (
	fakeFsRoot       = "/tmp/sriovnet-tests"
	pciSysDriversDir = "/sys/bus/pci/drivers"
)

func setupFakeFs(t *testing.T) func() {
	var err error
	var teardown func()
	utilfs.Fs, teardown, err = utilfs.NewFakeFs(fakeFsRoot)
	if err != nil {
		t.Errorf("setupFakeFs: Failed to create fake FS %v", err)
	}
	return teardown
}

func setupGetNetDevicesFromPciEnv(t *testing.T, pciAddress string, deviceNames []string) func() {
	var err error
	teardown := setupFakeFs(t)
	if len(deviceNames) > 0 {
		pciNetDir := filepath.Join(PciSysDir, pciAddress, "net")
		err = utilfs.Fs.MkdirAll(pciNetDir, os.FileMode(0755))
		defer func() {
			if err != nil {
				teardown()
				t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
			}
		}()
		for _, deviceName := range deviceNames {
			deviceNamePath := filepath.Join(pciNetDir, deviceName)
			err = utilfs.Fs.MkdirAll(deviceNamePath, os.FileMode(0755))
		}
	} else {
		pciNetDir := filepath.Join(PciSysDir, pciAddress)
		err = utilfs.Fs.MkdirAll(pciNetDir, os.FileMode(0755))
		if err != nil {
			teardown()
			t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
		}
	}
	return teardown
}

func TestGetNetDevicesFromPciSuccess(t *testing.T) {
	pciAddress := "0000:02:00.0"
	deviceNames := []string{"enp0s0f0", "enp0s0f1", "enp0s0f2"}
	teardown := setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	defer teardown()
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.NoError(t, err)
	assert.Equal(t, deviceNames, devNames)
}

func TestGetNetDevicesFromPciErrorNoPCI(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	pciAddress := "0000:02:00.0"
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, []string(nil), devNames)
}

func TestGetNetDevicesFromPciErrorNoDevices(t *testing.T) {
	var err error
	pciAddress := "0000:02:00.0"
	deviceNames := []string{}
	teardown := setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	defer teardown()
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, []string(nil), devNames)
}

func TestGetPfPciFromVfPci(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	// Create PCI sysfs layout with FakfeFs
	pfPciAddr := "0000:02:00.0"
	pfPciPath := filepath.Join(PciSysDir, pfPciAddr)
	vfPciAddr := "0000:02:00.6"
	vfPciPath := filepath.Join(PciSysDir, vfPciAddr)

	// PF PCI path
	_ = utilfs.Fs.MkdirAll(pfPciPath, os.FileMode(0755))
	// VF PCI path and physfn link
	_ = utilfs.Fs.MkdirAll(vfPciPath, os.FileMode(0755))
	_ = utilfs.Fs.Symlink(pfPciPath, filepath.Join(vfPciPath, "physfn"))

	pf, err := GetPfPciFromVfPci(vfPciAddr)
	assert.NoError(t, err)
	assert.Equal(t, pfPciAddr, pf)
}

func TestGetPfPciFromVfPciError(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	// Create PCI sysfs layout with FakfeFs
	pciAddr := "0000:02:00.0"
	pciPath := filepath.Join(PciSysDir, pciAddr)

	// PCI path
	_ = utilfs.Fs.MkdirAll(pciPath, os.FileMode(0755))

	pf, err := GetPfPciFromVfPci(pciAddr)
	assert.Error(t, err)
	assert.Equal(t, "", pf)
}

func TestIsVfPciVfioBound(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()

	// Create PCI sysfs layout with FakeFs. We want to achieve this:
	// /sys/bus/pci/devices/0000:02:00.0/driver -> ../../../../bus/pci/drivers/vfio-pci
	pciAddr := "0000:02:00.0"
	pciPath := filepath.Join(PciSysDir, pciAddr)
	vfioDriverPath := filepath.Join(pciSysDriversDir, "vfio-pci")

	_ = utilfs.Fs.MkdirAll(pciPath, os.FileMode(0755))
	_ = utilfs.Fs.MkdirAll(vfioDriverPath, os.FileMode(0755))
	symlinkTarget := filepath.Join(pciPath, "driver")
	_ = utilfs.Fs.Symlink(vfioDriverPath, symlinkTarget)

	vfioDevice := IsVfPciVfioBound(pciAddr)
	assert.Equal(t, true, vfioDevice)
}

func TestIsVfPciVfioBoundFalse(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()

	// Create PCI sysfs layout with FakeFs. We want to achieve this:
	// /sys/bus/pci/devices/0000:01:04.2/driver -> ../../../../bus/pci/drivers/mlx5_core
	pciAddr := "0000:01:04.2"
	pciPath := filepath.Join(PciSysDir, pciAddr)
	mlx5CoreDriverPath := filepath.Join(pciSysDriversDir, "mlx5_core")

	_ = utilfs.Fs.MkdirAll(mlx5CoreDriverPath, os.FileMode(0755))
	_ = utilfs.Fs.MkdirAll(pciPath, os.FileMode(0755))
	symlinkTarget := filepath.Join(pciPath, "driver")
	_ = utilfs.Fs.Symlink(mlx5CoreDriverPath, symlinkTarget)

	vfioDevice := IsVfPciVfioBound(pciAddr)
	assert.Equal(t, false, vfioDevice)
}

type devContext struct {
	Name    string
	PciAddr string
}

func setupGetPciFromNetDeviceEnv(t *testing.T, devices []*devContext) func() {
	var err error
	teardown := setupFakeFs(t)
	err = utilfs.Fs.MkdirAll(NetSysDir, os.FileMode(0755))
	defer func() {
		if err != nil {
			teardown()
			t.Errorf("setupGetPciFromNetDeviceEnv: got %v", err)
		}
	}()
	for _, dev := range devices {
		var symlinkTarget string
		symlinkName := filepath.Join(NetSysDir, dev.Name)
		if dev.PciAddr != "" {
			symlinkTarget = filepath.Join("/sys/devices/pci0000:00",
				dev.PciAddr, "net", dev.Name)
		} else {
			symlinkTarget = filepath.Join("/sys/devices/virtual/net", dev.Name)
		}
		err = utilfs.Fs.MkdirAll(symlinkTarget, os.FileMode(0755))
		if err != nil {
			return teardown
		}
		err = utilfs.Fs.Symlink(symlinkTarget, symlinkName)
		if err != nil {
			return teardown
		}
	}
	return teardown
}

func TestGetPciFromNetDevice(t *testing.T) {
	devices := []*devContext{
		{"p0", "0000:03:00.0"},
		{"pf0vf0", "0000:03:00.2"},
		{"pf0vf4", "0000:03:00.3"},
	}
	teardown := setupGetPciFromNetDeviceEnv(t, devices)
	defer teardown()

	pci, err := GetPciFromNetDevice(devices[0].Name)
	assert.NoError(t, err)
	assert.Equal(t, devices[0].PciAddr, pci)
}

func TestGetPciFromNetDeviceNotPCI(t *testing.T) {
	devices := []*devContext{
		{"br0", ""},
	}
	teardown := setupGetPciFromNetDeviceEnv(t, devices)
	defer teardown()

	_, err := GetPciFromNetDevice(devices[0].Name)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a PCI device")
}
