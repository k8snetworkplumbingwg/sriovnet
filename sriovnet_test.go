// +build integration

package sriovnet

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
)

func TestEnableSriov(t *testing.T) {

	err := EnableSriov("ib0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisableSriov(t *testing.T) {
	err := DisableSriov("ib0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetPfHandle(t *testing.T) {
	err1 := EnableSriov("ib0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ib0")
	if err2 != nil {
		t.Fatal(err2)
	}
	for _, vf := range handle.List {
		fmt.Printf("vf = %v\n", vf)
	}
}

func TestConfigVfs(t *testing.T) {
	err1 := EnableSriov("ens2f0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ens2f0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVfs(handle, false)
	if err3 != nil {
		t.Fatal(err3)
	}
	for _, vf := range handle.List {
		fmt.Printf("after config vf = %v\n", vf)
	}
}

func TestIsSriovEnabled(t *testing.T) {
	status := IsSriovEnabled("ens2f0")

	fmt.Printf("sriov status = %v", status)
}

func TestAllocFreeVf(t *testing.T) {
	var vfList [10]*VfObj

	err1 := EnableSriov("ib0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ib0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVfs(handle, false)
	if err3 != nil {
		t.Fatal(err3)
	}
	for i := 0; i < 10; i++ {
		vfList[i], _ = AllocateVf(handle)
	}
	for _, vf := range handle.List {
		fmt.Printf("after allocation vf = %v\n", vf)
	}
	for i := 0; i < 10; i++ {
		if vfList[i] == nil {
			continue
		}
		FreeVf(handle, vfList[i])
	}
	for _, vf := range handle.List {
		fmt.Printf("after free vf = %v\n", vf)
	}
}

func TestFreeByName(t *testing.T) {
	var vfList [10]*VfObj

	err1 := EnableSriov("ib0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ib0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVfs(handle, false)
	if err3 != nil {
		t.Fatal(err3)
	}
	for i := 0; i < 10; i++ {
		vfList[i], _ = AllocateVf(handle)
	}
	for _, vf := range handle.List {
		fmt.Printf("after allocation vf = %v\n", vf)
	}
	for i := 0; i < 10; i++ {
		if vfList[i] == nil {
			continue
		}
		err4 := FreeVfByNetdevName(handle, vfList[i].Index)
		if err4 != nil {
			t.Fatal(err4)
		}
	}
	for _, vf := range handle.List {
		fmt.Printf("after free vf = %v\n", vf)
	}
}

func TestAllocateVfByMac(t *testing.T) {
	var vfList [10]*VfObj
	var vfName [10]string

	err1 := EnableSriov("ens2f0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ens2f0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVfs(handle, true)
	if err3 != nil {
		t.Fatal(err3)
	}
	for i := 0; i < 10; i++ {
		vfList[i], _ = AllocateVf(handle)
		if vfList[i] != nil {
			vfName[i] = GetVfNetdevName(handle, vfList[i])
		}
	}
	for _, vf := range handle.List {
		fmt.Printf("after allocation vf = %v\n", vf)
	}
	for i := 0; i < 10; i++ {
		if vfList[i] == nil {
			continue
		}
		FreeVf(handle, vfList[i])
	}
	for _, vf := range handle.List {
		fmt.Printf("after alloc vf = %v\n", vf)
	}
	for i := 0; i < 2; i++ {
		if vfName[i] == "" {
			continue
		}
		mac, _ := GetVfDefaultMacAddr(vfName[i])
		vfList[i], _ = AllocateVfByMacAddress(handle, mac)
	}
	for _, vf := range handle.List {
		fmt.Printf("after alloc vf = %v\n", vf)
	}
}

func TestGetVfPciDevList(t *testing.T) {

	list, _ := GetVfPciDevList("ens2f0")
	fmt.Println("list is: ", list)
	t.Fatal(nil)
}

func TestGetVfNetdevName(t *testing.T) {
	var vfList [10]*VfObj
	var vfName [10]string

	handle, err2 := GetPfNetdevHandle("ens2f0")
	if err2 != nil {
		t.Fatal(err2)
	}
	for i := 0; i < 10; i++ {
		vfList[i], _ = AllocateVf(handle)
		if vfList[i] != nil {
			vfName[i] = GetVfNetdevName(handle, vfList[i])
			t.Log("Allocated VF: ", vfList[i].Index, "Netdev: ", vfName[i])
		}
	}
}

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
	return func() { utilfs.Fs.RemoveAll("/") }
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
