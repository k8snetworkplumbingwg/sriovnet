package sriovnet

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
	"github.com/Mellanox/sriovnet/pkg/utils/netlinkops"
	netlinkopsMocks "github.com/Mellanox/sriovnet/pkg/utils/netlinkops/mocks"
)

type repContext struct {
	Name         string // create files /sys/bus/pci/devices/<vf addr>/physfn/net/<Name> , /sys/class/net/<Name>
	PhysPortName string // conditionally create if string is empty under /sys/class/net/<Name>/phys_port_name
	PhysSwitchID string // conditionally create if string is empty under /sys/class/net/<Name>/phys_switch_id
}

// setUpRepPhysFiles sets up phys_port_name and phys_switch_id files for specified representor
// Note: should not be called directly as it expects FakeFs and representor
// path to be initialized beforehand.
func setUpRepPhysFiles(rep *repContext) error {
	var err error

	if rep.PhysPortName != "" {
		physPortNamePath := filepath.Join(NetSysDir, rep.Name, netdevPhysPortName)
		physPortNameFile, _ := utilfs.Fs.Create(physPortNamePath)
		_, err = physPortNameFile.Write([]byte(rep.PhysPortName))
		if err != nil {
			return err
		}
	}

	if rep.PhysSwitchID != "" {
		physSwitchIDPath := filepath.Join(NetSysDir, rep.Name, netdevPhysSwitchID)
		physSwitchIDFile, _ := utilfs.Fs.Create(physSwitchIDPath)
		_, err = physSwitchIDFile.Write([]byte(rep.PhysSwitchID))
		if err != nil {
			return err
		}
	}

	return nil
}

// setUpRepresentorLayout sets up the representor filesystem layout.
// Note: should not be called directly as it expects FakeFs to be initialized beforehand.
func setUpRepresentorLayout(vfPciAddress string, rep *repContext) error {
	// This method assumes FakeFs it already set up
	_, ok := utilfs.Fs.(*utilfs.FakeFs)
	if !ok {
		return fmt.Errorf("fakeFs was not initialized")
	}

	if vfPciAddress != "" {
		path := filepath.Join(PciSysDir, vfPciAddress, "physfn", "net", rep.Name)
		err := utilfs.Fs.MkdirAll(path, os.FileMode(0755))
		if err != nil {
			return err
		}
	}

	path := filepath.Join(NetSysDir, rep.Name)
	err := utilfs.Fs.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		return err
	}

	return setUpRepPhysFiles(rep)
}

//nolint:unparam
// setupUplinkRepresentorEnv sets up the uplink representor and related VF representors filesystem layout.
func setupUplinkRepresentorEnv(t *testing.T, uplink *repContext, vfPciAddress string, vfReps []*repContext) func() {
	var err error
	teardown := setupRepresentorEnv(t, vfPciAddress, vfReps)
	defer func() {
		if err != nil {
			teardown()
			t.Errorf("setupUplinkRepresentorEnv, got %v", err)
		}
	}()
	// Setup uplink
	err = setUpRepresentorLayout(vfPciAddress, uplink)
	if err != nil {
		return nil
	}

	return teardown
}

// setupRepresentorEnv sets up VF representors filesystem layout.
func setupRepresentorEnv(t *testing.T, vfPciAddress string, vfReps []*repContext) func() {
	var err error
	teardown := setupFakeFs(t)

	defer func() {
		if err != nil {
			teardown()
			t.Errorf("setupRepresentorEnv, got %v", err)
		}
	}()

	for _, rep := range vfReps {
		err = setUpRepresentorLayout(vfPciAddress, rep)
	}

	return teardown
}

// setupDPUConfigFileForPort sets the config file content for a specific DPU port of a given uplink
func setupDPUConfigFileForPort(t *testing.T, uplink, portName, fileContent string) {
	// This method assumes FakeFs it already set up
	assert.IsType(t, &utilfs.FakeFs{}, utilfs.Fs)

	path := filepath.Join(NetSysDir, uplink, "smart_nic", portName)
	err := utilfs.Fs.MkdirAll(path, os.FileMode(0755))
	assert.NoError(t, err)

	repConfigFilePath := filepath.Join(path, "config")
	repConfigFileName, _ := utilfs.Fs.Create(repConfigFilePath)
	_, err = repConfigFileName.Write([]byte(fileContent))
	assert.NoError(t, err)
}

func TestGetUplinkRepresentorWithPhysPortNameSuccess(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{"eth0", "p0", "111111"}
	vfsReps := []*repContext{{"enp_0", "pf0vf0", "0123"},
		{"enp_1", "pf0vf1", "0124"}}

	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", uplinkNetdev)
}

func TestGetUplinkRepresentorWithoutPhysPortNameSuccess(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{Name: "eth0", PhysSwitchID: "111111"}
	var vfsReps []*repContext

	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", uplinkNetdev)
}

func TestGetUplinkRepresentorForPfSuccess(t *testing.T) {
	pfPciAddress := "0000:03:00.0"
	uplinkRep := &repContext{"eth0", "p0", "111111"}

	vfPciAddress := ""
	var vfsReps []*repContext

	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	_ = utilfs.Fs.MkdirAll(filepath.Join(PciSysDir, pfPciAddress, "net", uplinkRep.Name), os.FileMode(0755))
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(pfPciAddress)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", uplinkNetdev)
}

func TestGetUplinkRepresentorWithPhysPortNameFailed(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{"eth0", "invalid", "111111"}
	vfsReps := []*repContext{{"enp_0", "pf0vf0", "0123"},
		{"enp_1", "pf0vf1", "0124"}}

	expectedError := fmt.Sprintf("uplink for %s not found", vfPciAddress)
	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Equal(t, expectedError, err.Error())
}

func TestGetUplinkRepresentorErrorMissingSwID(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{Name: "eth0", PhysPortName: "p0"}
	vfsReps := []*repContext{{Name: "enp_0", PhysPortName: "pf0vf0"},
		{Name: "enp_1", PhysPortName: "pf0vf1"}}
	expectedError := fmt.Sprintf("uplink for %s not found", vfPciAddress)
	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Equal(t, expectedError, err.Error())
}

func TestGetUplinkRepresentorErrorEmptySwID(t *testing.T) {
	var testErr error
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{"eth0", "", ""}
	var vfsReps []*repContext
	expectedError := fmt.Sprintf("uplink for %s not found", vfPciAddress)
	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	defer teardown()
	swIDFile := filepath.Join(NetSysDir, "eth0", netdevPhysSwitchID)
	swID, testErr := utilfs.Fs.Create(swIDFile)
	defer func() {
		if testErr != nil {
			t.Errorf("setupUplinkRepresentorEnv, got %v", testErr)
		}
	}()
	_, testErr = swID.Write([]byte(""))
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Equal(t, expectedError, err.Error())
}

func TestGetUplinkRepresentorErrorMissingUplink(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	expectedError := fmt.Sprintf("failed to lookup %s", vfPciAddress)
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetVfRepresentorDPU(t *testing.T) {
	vfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "pf0vf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf0vf1",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth2",
			PhysPortName: "pf0vf2",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupRepresentorEnv(t, "", vfReps)
	defer teardown()

	vfRep, err := GetVfRepresentorDPU("0", "2")
	assert.NoError(t, err)
	assert.Equal(t, "eth2", vfRep)
}

func setupSfRepresentorEnv(t *testing.T, sfReps []*repContext) func() {
	var err error
	teardown := setupFakeFs(t)

	defer func() {
		if err != nil {
			teardown()
			t.Errorf("setupSfRepresentorEnv, got %v", err)
		}
	}()

	pfNetPath := filepath.Join(NetSysDir, "p0", "device", "net")
	err = utilfs.Fs.MkdirAll(pfNetPath, os.FileMode(0755))
	if err != nil {
		return nil
	}
	for _, rep := range sfReps {
		repPath := filepath.Join(pfNetPath, rep.Name)
		repLink := filepath.Join(NetSysDir, rep.Name)

		err = utilfs.Fs.MkdirAll(repPath, os.FileMode(0755))
		if err != nil {
			break
		}

		_ = utilfs.Fs.Symlink(repPath, repLink)
		if err = setUpRepPhysFiles(rep); err != nil {
			break
		}
	}

	return teardown
}

func TestGetSfRepresentorSuccess(t *testing.T) {
	sfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "pf0sf0",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf0sf1",
		},
		{
			Name:         "eth2",
			PhysPortName: "pf0sf2",
		},
	}
	teardown := setupSfRepresentorEnv(t, sfReps)
	defer teardown()

	sfRep, err := GetSfRepresentor("p0", 2)
	assert.NoError(t, err)
	assert.Equal(t, "eth2", sfRep)
}

func TestGetSfRepresentorErrorNoRep(t *testing.T) {
	sfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "pf0sf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf0sf1",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth2",
			PhysPortName: "pf0sf2",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupSfRepresentorEnv(t, sfReps)
	expectedError := "failed to find SF representor for uplink p0"
	defer teardown()

	sfRep, err := GetSfRepresentor("p0", 3)
	assert.Error(t, err)
	assert.Equal(t, "", sfRep)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetSfRepresentorErrorNotExistingUplink(t *testing.T) {
	sfReps := []*repContext{}
	teardown := setupSfRepresentorEnv(t, sfReps)
	expectedError := "no such file or directory"
	defer teardown()

	sfRep, err := GetSfRepresentor("p1", 0)
	assert.Error(t, err)
	assert.Equal(t, "", sfRep)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetPortIndexFromRepresentor(t *testing.T) {
	vfReps := []*repContext{
		{
			Name:         "p0",
			PhysPortName: "p0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "pf0hpf",
			PhysPortName: "pf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "pf0vf10",
			PhysPortName: "pf0vf10",
			PhysSwitchID: "fc10d80003a1420c",
		},
		{
			Name:         "pf0sf50",
			PhysPortName: "pf0sf50",
			PhysSwitchID: "fc10d80003a1420c",
		},
		{
			Name:         "eth3",
			PhysPortName: "",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "noswitchdev",
			PhysPortName: "",
			PhysSwitchID: "",
		},
	}
	teardown := setupRepresentorEnv(t, "", vfReps)
	defer teardown()

	tcases := []struct {
		netdev        string
		expectedID    int
		expectedError string
		shouldFail    bool
	}{
		{netdev: "pf0vf10", expectedID: 10, expectedError: "", shouldFail: false},
		{netdev: "pf0sf50", expectedID: 50, expectedError: "", shouldFail: false},
		{netdev: "p0", expectedID: 0, expectedError: "unsupported port flavor", shouldFail: true},
		{netdev: "pf0hpf", expectedID: 0, expectedError: "unsupported port flavor", shouldFail: true},
		{netdev: "eth3", expectedID: 0, expectedError: "no such file or directory", shouldFail: true},
		{netdev: "notswitchdev", expectedID: 0, expectedError: "does not represent an eswitch port", shouldFail: true},
	}

	for _, tcase := range tcases {
		portID, err := GetPortIndexFromRepresentor(tcase.netdev)
		if tcase.shouldFail {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tcase.expectedError)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, portID, tcase.expectedID)
		}
	}
}

func TestGetVfRepresentorDPUNoRep(t *testing.T) {
	vfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "pf0vf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf0vf1",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupRepresentorEnv(t, "", vfReps)
	defer teardown()

	vfRep, err := GetVfRepresentorDPU("1", "2")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}

func TestGetVfRepresentorDPUInvalidPfID(t *testing.T) {
	vfRep, err := GetVfRepresentorDPU("invalid", "2")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}

func TestGetVfRepresentorDPUInvalidVfIndex(t *testing.T) {
	vfRep, err := GetVfRepresentorDPU("1", "invalid")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}

func TestGetSfRepresentorDPUSuccess(t *testing.T) {
	sfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "pf1sf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf1sf1",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth2",
			PhysPortName: "pf1sf2",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupSfRepresentorEnv(t, sfReps)
	defer teardown()
	sfRep, err := GetSfRepresentorDPU("1", "1")
	assert.NoError(t, err)
	assert.Equal(t, "eth1", sfRep)
}

func TestGetSfRepresentorDPUErrorNoRep(t *testing.T) {
	sfReps := []*repContext{
		{PhysPortName: "pf1sf0"},
		{PhysPortName: "pf1sf1"},
	}
	teardown := setupSfRepresentorEnv(t, sfReps)
	defer teardown()

	sfRep, err := GetSfRepresentorDPU("1", "2")
	assert.Error(t, err)
	assert.Equal(t, "", sfRep)
}

func TestGetSfRepresentorDPUErrorInvalidPfID(t *testing.T) {
	sfRep, err := GetSfRepresentorDPU("invalid", "3")
	assert.Error(t, err)
	assert.Equal(t, "", sfRep)
}

func TestGetSfRepresentorDPUErrorInvalidSfIndex(t *testing.T) {
	sfRep, err := GetSfRepresentorDPU("1", "invalid")
	assert.Error(t, err)
	assert.Equal(t, "", sfRep)
}

func TestGetVfRepresentorPortFlavour(t *testing.T) {
	vfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "p0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth1",
			PhysPortName: "pf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth2",
			PhysPortName: "pf0vf1",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth44",
			PhysPortName: "pf0sf44",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "eth10",
			PhysPortName: "unknown",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupRepresentorEnv(t, "", vfReps)
	defer teardown()

	tcases := []struct {
		netdev     string
		expected   PortFlavour
		shouldFail bool
	}{
		{netdev: "eth0", expected: PORT_FLAVOUR_PHYSICAL, shouldFail: false},
		{netdev: "eth1", expected: PORT_FLAVOUR_PCI_PF, shouldFail: false},
		{netdev: "eth2", expected: PORT_FLAVOUR_PCI_VF, shouldFail: false},
		{netdev: "eth44", expected: PORT_FLAVOUR_PCI_SF, shouldFail: false},
		{netdev: "eth10", expected: PORT_FLAVOUR_UNKNOWN, shouldFail: false},
		{netdev: "foobar", expected: PORT_FLAVOUR_UNKNOWN, shouldFail: true},
	}

	defer netlinkops.ResetNetlinkOps()
	for _, tcase := range tcases {
		nlOpsMock := netlinkopsMocks.NetlinkOps{}
		netlinkops.SetNetlinkOps(&nlOpsMock)
		nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
			nil, fmt.Errorf("failed to get devlink port"))
		f, err := GetRepresentorPortFlavour(tcase.netdev)
		if tcase.shouldFail {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		assert.Equal(t, tcase.expected, f)
	}
}

func TestGetVfRepresentorPortFlavourDevlink(t *testing.T) {
	nlOpsMock := netlinkopsMocks.NetlinkOps{}
	netlinkops.SetNetlinkOps(&nlOpsMock)
	defer netlinkops.ResetNetlinkOps()

	teardown := setupRepresentorEnv(t, "", []*repContext{{
		Name:         "enp3s0f0_0",
		PhysPortName: "pf0vf0",
		PhysSwitchID: "c2cfc60003a1420c",
	}})
	defer teardown()

	nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
		&netlink.DevlinkPort{
			BusName:       "pci",
			DeviceName:    "0000:03:00.0",
			PortIndex:     126654,
			PortType:      2, // ETH
			NetdeviceName: "enp3s0f0_0",
			PortFlavour:   PORT_FLAVOUR_PCI_VF,
			Fn:            nil,
		}, nil)

	f, err := GetRepresentorPortFlavour("enp3s0f0_0")
	assert.NoError(t, err)
	assert.Equal(t, PortFlavour(PORT_FLAVOUR_PCI_VF), f)
}

func TestGetRepresentorPeerMacAddress(t *testing.T) {
	// Create uplink and PF representor relate files
	vfReps := []*repContext{
		{
			Name:         "eth0",
			PhysPortName: "p0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "pf0hpf",
			PhysPortName: "pf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "rep_0",
			PhysPortName: "pf0vf0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	}
	teardown := setupRepresentorEnv(t, "", vfReps)
	defer teardown()
	defer netlinkops.ResetNetlinkOps()

	// Create PF representor config file
	repConfigFile := `
MAC        : 0c:42:a1:de:cf:7c
MaxTxRate  : 0
State      : Follow
`
	setupDPUConfigFileForPort(t, "eth0", "pf", repConfigFile)
	// Run test
	tcases := []struct {
		netdev      string
		expectedMac string
		shouldFail  bool
	}{
		{netdev: "pf0hpf", expectedMac: "0c:42:a1:de:cf:7c", shouldFail: false},
		{netdev: "rep_0", expectedMac: "", shouldFail: true},
		{netdev: "p0", expectedMac: "", shouldFail: true},
		{netdev: "foobar", expectedMac: "", shouldFail: true},
	}

	for _, tcase := range tcases {
		nlOpsMock := netlinkopsMocks.NetlinkOps{}
		netlinkops.SetNetlinkOps(&nlOpsMock)
		nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
			nil, fmt.Errorf("failed to get devlink port"))

		mac, err := GetRepresentorPeerMacAddress(tcase.netdev)
		if tcase.shouldFail {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tcase.expectedMac, mac.String())
		}
	}
}

func TestGetRepresentorPeerMacAddressDevlink(t *testing.T) {
	nlOpsMock := netlinkopsMocks.NetlinkOps{}
	netlinkops.SetNetlinkOps(&nlOpsMock)
	defer netlinkops.ResetNetlinkOps()

	teardown := setupRepresentorEnv(t, "", []*repContext{{
		Name:         "pf0hpf",
		PhysPortName: "pf0",
		PhysSwitchID: "c2cfc60003a1420c",
	}})
	defer teardown()

	dlport := netlink.DevlinkPort{
		BusName:       "pci",
		DeviceName:    "0000:03:00.0",
		PortIndex:     126654,
		PortType:      2, // ETH
		NetdeviceName: "pf0hpf",
		PortFlavour:   PORT_FLAVOUR_PCI_PF,
		Fn:            &netlink.DevlinkPortFn{HwAddr: net.HardwareAddr{0x0c, 0x42, 0xa1, 0xde, 0xcf, 0x7c}},
	}
	nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(&dlport, nil)
	nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(&dlport, nil)

	mac, err := GetRepresentorPeerMacAddress("pf0hpf")
	assert.NoError(t, err)
	assert.Equal(t, "0c:42:a1:de:cf:7c", mac.String())
}

func TestSetRepresentorPeerMacAddress(t *testing.T) {
	nlOpsMock := netlinkopsMocks.NetlinkOps{}
	netlinkops.SetNetlinkOps(&nlOpsMock)
	defer netlinkops.ResetNetlinkOps()

	teardown := setupRepresentorEnv(t, "", []*repContext{
		{
			Name:         "pf0vf24",
			PhysPortName: "pf0vf24",
			PhysSwitchID: "c2cfc60003a1420c",
		},
		{
			Name:         "p0",
			PhysPortName: "p0",
			PhysSwitchID: "c2cfc60003a1420c",
		},
	})
	defer teardown()

	// Create PCI sysfs layout with FakeFs. We want to achieve this:
	// /sys/class/net
	pfID := "0"
	vfIdx := "24"
	mac := net.HardwareAddr{0, 0, 0, 1, 2, 3}

	path := fmt.Sprintf("%s/p%s/smart_nic/vf%s", NetSysDir, pfID, vfIdx)
	_ = utilfs.Fs.MkdirAll(path, os.FileMode(0755))

	macFile := filepath.Join(path, "mac")
	_, _ = utilfs.Fs.Create(macFile)

	nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(nil, fmt.Errorf("no devlink support"))
	err := SetRepresentorPeerMacAddress("pf0vf24", mac)
	assert.NoError(t, err)
}
