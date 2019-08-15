package sriovnet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
)

func setupGetNetDevicesFromPciEnv(t *testing.T, pciAddress string, deviceNames []string) func() {
	var err error
	utilfs.Fs = utilfs.NewFakeFs()
	pciNetDir := filepath.Join(PciSysDir, pciAddress, "net")
	err = utilfs.Fs.MkdirAll(pciNetDir, os.FileMode(0755))
	defer func() {
		if err != nil {
			t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
		}
	}()
	for _, deviceName := range deviceNames {
		deviceNamePath := filepath.Join(pciNetDir, deviceName)
		err = utilfs.Fs.MkdirAll(deviceNamePath, os.FileMode(0755))
		if err != nil {
			t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
		}
	}
	return func() {_ = utilfs.Fs.RemoveAll("/")}
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
	var err error
	pciAddress := "0000:02:00.0"
	deviceNames := []string{"enp0s0f0", "enp0s0f1", "enp0s0f2"}
	expectedError := fmt.Sprintf("cannot get a network device with pci "+
		"address %s open /sys/bus/pci/devices/%s/net: file does not exist", pciAddress, pciAddress)
	teardown := setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	defer teardown()
	pciNetDir := filepath.Join(PciSysDir, pciAddress, "net")
	err = utilfs.Fs.RemoveAll(pciNetDir)
	if err != nil {
		t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
	}
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err.Error())
	assert.Equal(t, []string(nil), devNames)
}

func TestGetNetDevicesFromPciErrorNoDevices(t *testing.T) {
	var err error
	pciAddress := "0000:02:00.0"
	deviceNames := []string{"enp0s0f0", "enp0s0f1", "enp0s0f2"}
	expectedError := fmt.Sprintf("failed to get network device name in /sys/bus/pci/devices/%s/"+
		"net readdir /sys/bus/pci/devices/%s/net: not a dir", pciAddress, pciAddress)
	teardown := setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	defer teardown()

	pciNetDir := filepath.Join(PciSysDir, pciAddress, "net")
	err = utilfs.Fs.RemoveAll(pciNetDir)
	if err != nil {
		t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
	}
	_, err = utilfs.Fs.Create(pciNetDir)
	if err != nil {
		t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
	}
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err.Error())
	assert.Equal(t, []string(nil), devNames)
}
