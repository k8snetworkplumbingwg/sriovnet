package sriovnet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
)

func setupGetNetDevicesFromPciEnv(t *testing.T, pciAddress string, deviceNames []string) {
	var err error
	utilfs.Fs = utilfs.NewFakeFs()
	if len(deviceNames) > 0 {
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
		}
	} else {
		pciNetDir := filepath.Join(PciSysDir, pciAddress)
		err = utilfs.Fs.MkdirAll(pciNetDir, os.FileMode(0755))
		if err != nil {
			t.Errorf("setupGetNetDevicesFromPciEnv, got %v", err)
		}
	}
}

func TestGetNetDevicesFromPciSuccess(t *testing.T) {
	pciAddress := "0000:02:00.0"
	deviceNames := []string{"enp0s0f0", "enp0s0f1", "enp0s0f2"}
	setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.NoError(t, err)
	assert.Equal(t, deviceNames, devNames)
}

func TestGetNetDevicesFromPciErrorNoPCI(t *testing.T) {
	var err error // use new fakeFs
	pciAddress := "0000:02:00.0"
	utilfs.Fs = utilfs.NewFakeFs()
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, []string(nil), devNames)
}

func TestGetNetDevicesFromPciErrorNoDevices(t *testing.T) {
	var err error
	pciAddress := "0000:02:00.0"
	deviceNames := []string{}
	setupGetNetDevicesFromPciEnv(t, pciAddress, deviceNames)
	devNames, err := GetNetDevicesFromPci(pciAddress)
	assert.Error(t, err)
	assert.Equal(t, []string(nil), devNames)
}
