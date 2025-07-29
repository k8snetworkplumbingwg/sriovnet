/*
Copyright 2022 NVIDIA CORPORATION &

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sriovnet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/k8snetworkplumbingwg/sriovnet/pkg/utils/filesystem"
)

type auxDevContext struct {
	parent string
	sfNum  string
	name   string
}

// setupAuxDevEnv creates (fake) auxiliary devices, returns error if setup failed.
func setUpAuxDevEnv(t *testing.T, auxDevs []auxDevContext) {
	var err error

	err = utilfs.Fs.MkdirAll(AuxSysDir, os.FileMode(0755))
	assert.NoError(t, err)

	for _, dev := range auxDevs {
		auxDevPathPCI := filepath.Join(PciSysDir, dev.parent, dev.name)
		auxDevPathAux := filepath.Join(AuxSysDir, dev.name)

		err = utilfs.Fs.MkdirAll(auxDevPathPCI, os.FileMode(0755))
		assert.NoError(t, err)
		if dev.sfNum != "" {
			err = utilfs.Fs.WriteFile(filepath.Join(auxDevPathPCI, "sfnum"), []byte(dev.sfNum), os.FileMode(0655))
			assert.NoError(t, err)
		}
		err = utilfs.Fs.Symlink(auxDevPathPCI, auxDevPathAux)
		assert.NoError(t, err)
	}
}

func TestGetNetDevicesFromAuxSuccess(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"
	netDevName := "en3f0pf0sf0"
	path := filepath.Join(AuxSysDir, auxDevName, "net", netDevName)
	_ = utilfs.Fs.MkdirAll(path, os.FileMode(0755))

	devNames, err := GetNetDevicesFromAux(auxDevName)
	assert.NoError(t, err)
	assert.Equal(t, netDevName, devNames[0])
}

func TestGetNetDevicesFromAuxErrorNoAux(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"
	devNames, err := GetNetDevicesFromAux(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, ([]string(nil)), devNames)
}

func TestGetNetDevicesFromAuxErrorNoDevice(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"

	_ = utilfs.Fs.MkdirAll(filepath.Join(AuxSysDir, auxDevName, "net"), os.FileMode(0755))
	devNames, _ := GetNetDevicesFromAux(auxDevName)
	assert.Equal(t, ([]string{}), devNames)
}

func TestGetSfIndexByAuxDevSuccess(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"
	auxDevPath := filepath.Join(AuxSysDir, auxDevName)
	sfNumFile := filepath.Join(auxDevPath, "sfnum")

	_ = utilfs.Fs.MkdirAll(auxDevPath, os.FileMode(0755))
	_ = utilfs.Fs.WriteFile(sfNumFile, []byte("0"), os.FileMode(0644))

	sfIndex, err := GetSfIndexByAuxDev(auxDevName)
	assert.NoError(t, err)
	assert.Equal(t, 0, sfIndex)
}

func TestGetSfIndexByAuxDevErrorNoSfNum(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"
	expectedError := "cannot get sfnum"

	sfIndex, err := GetSfIndexByAuxDev(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, -1, sfIndex)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetSfIndexByAuxDevErrorRead(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.19"
	auxDevPath := filepath.Join(AuxSysDir, auxDevName)
	sfNumFile := filepath.Join(auxDevPath, "sfnum1")
	expectedError := "cannot get sfnum"

	_ = utilfs.Fs.MkdirAll(auxDevPath, os.FileMode(0755))
	_ = utilfs.Fs.WriteFile(sfNumFile, []byte("0"), os.FileMode(0))

	sfIndex, err := GetSfIndexByAuxDev(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, -1, sfIndex)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetSfIndexByAuxDevErrorAtoi(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	auxDevName := "mlx5_core.sf.0"
	auxDevPath := filepath.Join(AuxSysDir, auxDevName)
	sfNumFile := filepath.Join(auxDevPath, "sfnum")
	expectedError := "invalid syntax"

	_ = utilfs.Fs.MkdirAll(auxDevPath, os.FileMode(0755))
	_ = utilfs.Fs.WriteFile(sfNumFile, []byte("NaN"), os.FileMode(0644))

	sfIndex, err := GetSfIndexByAuxDev(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, -1, sfIndex)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetPfPciFromAux(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	// Create PCI & Auxiliary sysfs layout with FakeFs
	pfPciAddr := "0000:02:00.0"
	auxDevName := "mlx5_core.eth.0"
	auxDevPath := filepath.Join(PciSysDir, pfPciAddr, auxDevName)
	auxDevLink := filepath.Join(AuxSysDir, auxDevName)

	// PF PCI path and auxiliary device dir
	_ = utilfs.Fs.MkdirAll(auxDevPath, os.FileMode(0755))
	_ = utilfs.Fs.MkdirAll(AuxSysDir, os.FileMode(0755))
	// Auxiliary device link
	_ = utilfs.Fs.Symlink(auxDevPath, auxDevLink)

	pf, err := GetPfPciFromAux(auxDevName)
	assert.NoError(t, err)
	assert.Equal(t, pfPciAddr, pf)
}

func TestGetPfPciFromAuxNoSuchDevice(t *testing.T) {
	// Create PCI sysfs layout with FakeFs
	auxDevName := "mlx5_core.eth.0"

	pf, err := GetPfPciFromAux(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, "", pf)
}

func TestGetUplinkRepresentorFromAux(t *testing.T) {
	teardownFs := setupFakeFs(t)
	defer teardownFs()
	// Create PCI & Auxiliary sysfs layout with FakeFs
	pfPciAddr := "0000:02:00.0"
	auxDevName := "mlx5_core.eth.0"
	auxDevPath := filepath.Join(PciSysDir, pfPciAddr, auxDevName)
	auxDevLink := filepath.Join(AuxSysDir, auxDevName)

	uplinkRep := &repContext{"eth0", "p0", "111111"}
	sfsReps := []*repContext{{"enp_0", "pf0sf0", "0123"}}

	teardownUplink := setupUplinkRepresentorEnv(t, uplinkRep, pfPciAddr, sfsReps)
	defer teardownUplink()

	// PF PCI path and auxiliary device dir
	_ = utilfs.Fs.MkdirAll(auxDevPath, os.FileMode(0755))
	_ = utilfs.Fs.MkdirAll(AuxSysDir, os.FileMode(0755))
	// Auxiliary device link
	_ = utilfs.Fs.Symlink(auxDevPath, auxDevLink)

	pf, err := GetUplinkRepresentorFromAux(auxDevName)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", pf)
}

func TestGetUplinkRepresentorFromAuxNoSuchDevice(t *testing.T) {
	// Create PCI sysfs layout with FakeFs
	auxDevName := "mlx5_core.eth.0"

	pf, err := GetUplinkRepresentorFromAux(auxDevName)
	assert.Error(t, err)
	assert.Equal(t, "", pf)
}

func createPciDevicePaths(t *testing.T, pciAddr string, dirs []string) {
	for _, dir := range dirs {
		path := filepath.Join(PciSysDir, pciAddr, dir)
		assert.NoError(t, utilfs.Fs.MkdirAll(path, os.FileMode(0755)))
	}
}

func TestGetAuxNetDevicesFromPciSuccess(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	pciAddr := "0000:00:01.0"
	devs := []string{"foo.bar.0", "foo.bar.1", "foo.baz.0"}

	createPciDevicePaths(t, pciAddr, devs)
	// create few regular directories
	createPciDevicePaths(t, pciAddr, []string{"infiniband", "net"})

	auxDevs, err := GetAuxNetDevicesFromPci(pciAddr)
	assert.NoError(t, err)
	assert.Equal(t, auxDevs, devs)
}

func TestGetAuxNetDevicesFromPciSuccessNoDevices(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	pciAddr := "0000:00:01.0"

	createPciDevicePaths(t, pciAddr, []string{"infiniband", "net"})

	auxDevs, err := GetAuxNetDevicesFromPci(pciAddr)
	assert.NoError(t, err)
	assert.Equal(t, auxDevs, []string{})
}

func TestGetAuxNetDevicesFromPciFailureNoSuchDevice(t *testing.T) {
	pciAddr := "0000:00:01.0"
	auxDevs, err := GetAuxNetDevicesFromPci(pciAddr)
	assert.Error(t, err)
	assert.Equal(t, auxDevs, []string(nil))
}

func TestGetAuxNetDevicesFromPciFailureNotANetworkDevice(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()
	pciAddr := "0000:00:01.0"

	createPciDevicePaths(t, pciAddr, []string{"infiniband"})

	auxDevs, err := GetAuxNetDevicesFromPci(pciAddr)
	assert.Error(t, err)
	assert.Equal(t, auxDevs, []string(nil))
}

func TestGetAuxSFDevByPciAndSFIndex(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()

	pciAddr := "0000:03:00.0"
	devs := []auxDevContext{
		{
			parent: pciAddr,
			sfNum:  "",
			name:   "mlx5_core.eth.0",
		},
		{
			parent: pciAddr,
			sfNum:  "",
			name:   "mlx5_core.eth-rep.0",
		},
		{
			parent: pciAddr,
			sfNum:  "123",
			name:   "mlx5_core.sf.3",
		},
	}
	setUpAuxDevEnv(t, devs)
	createPciDevicePaths(t, pciAddr, []string{"infiniband", "net"})

	device, err := GetAuxSFDevByPciAndSFIndex(pciAddr, 123)
	assert.NoError(t, err)
	assert.Equal(t, "mlx5_core.sf.3", device)
}

func TestGetAuxSFDevByPciAndSFIndexSFIndexNotFound(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()

	pciAddr := "0000:03:00.0"
	devs := []auxDevContext{
		{
			parent: pciAddr,
			sfNum:  "123",
			name:   "mlx5_core.sf.3",
		},
	}
	setUpAuxDevEnv(t, devs)
	createPciDevicePaths(t, pciAddr, []string{"infiniband", "net"})

	device, err := GetAuxSFDevByPciAndSFIndex(pciAddr, 122)
	assert.Error(t, err)
	assert.Equal(t, ErrDeviceNotFound, err)
	assert.Equal(t, "", device)
}

func TestGetAuxSFDevByPciAndSFIndexPCIAddressNotFound(t *testing.T) {
	teardown := setupFakeFs(t)
	defer teardown()

	createPciDevicePaths(t, "0000:03:00.0", []string{"infiniband", "net"})

	_, err := GetAuxSFDevByPciAndSFIndex("0000:04:00.0", 4)
	assert.Error(t, err)
	assert.NotEqual(t, ErrDeviceNotFound, err)
}
