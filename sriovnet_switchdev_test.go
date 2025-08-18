/*
Copyright 2023 NVIDIA CORPORATION & AFFILIATES

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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	utilfs "github.com/k8snetworkplumbingwg/sriovnet/pkg/utils/filesystem"
	"github.com/k8snetworkplumbingwg/sriovnet/pkg/utils/netlinkops"
	netlinkopsMocks "github.com/k8snetworkplumbingwg/sriovnet/pkg/utils/netlinkops/mocks"
)

type repContext struct {
	// Name is the representor netdev name
	Name string // create files /sys/bus/pci/devices/<vf addr>/physfn/net/<Name> , /sys/class/net/<Name>
	// PhysPortName is the phys_port_name of the representor netdev
	PhysPortName string // conditionally create if non empty under /sys/class/net/<Name>/phys_port_name
	// PhysSwitchID is the phys_switch_id of the representor netdev
	PhysSwitchID string // conditionally create if non empty under /sys/class/net/<Name>/phys_switch_id
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

// setupUplinkRepresentorEnv sets up the uplink representor and related VF representors filesystem layout.
//
//nolint:unparam
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
	if uplink != nil {
		err = setUpRepresentorLayout(vfPciAddress, uplink)
		if err != nil {
			teardown()
			t.Errorf("setupUplinkRepresentorEnv, got %v", err)
		}
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
		if err != nil {
			t.Errorf("setupRepresentorEnv, got %v", err)
		}
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

func setupRepresentorEnvForGetVfRepresentor(t *testing.T, uplink *repContext, vfReps []*repContext) func() {
	var err error
	teardown := setupFakeFs(t)

	defer func() {
		if err != nil {
			teardown()
			t.Errorf("setupRepresentorEnvForGetVfRepresentor, got %v", err)
		}
	}()

	// Create the uplink device/net directory structure
	pfNetPath := filepath.Join(NetSysDir, uplink.Name, "device", "net")
	err = utilfs.Fs.MkdirAll(pfNetPath, os.FileMode(0755))
	if err != nil {
		teardown()
		t.Errorf("setupRepresentorEnvForGetVfRepresentor, got %v", err)
	}

	for _, rep := range vfReps {
		// Create representor directory under the uplink's device/net path
		repPath := filepath.Join(pfNetPath, rep.Name)
		repLink := filepath.Join(NetSysDir, rep.Name)

		err = utilfs.Fs.MkdirAll(repPath, os.FileMode(0755))
		if err != nil {
			teardown()
			t.Errorf("setupRepresentorEnvForGetVfRepresentor, got %v", err)
		}

		// Create symlink from /sys/class/net/<rep_name> to the rep path
		_ = utilfs.Fs.Symlink(repPath, repLink)

		if err = setUpRepPhysFiles(rep); err != nil {
			teardown()
			t.Errorf("setupRepresentorEnvForGetVfRepresentor, got %v", err)
		}
	}

	// create phys_port_name and phys_switch_id files for the uplink
	if err = setUpRepPhysFiles(uplink); err != nil {
		teardown()
		t.Errorf("setupRepresentorEnvForGetVfRepresentor, got %v", err)
	}

	return teardown
}

func TestGetVfRepresentor(t *testing.T) {
	tcases := []struct {
		name          string
		uplink        *repContext
		vfReps        []*repContext
		vfIndex       int
		expectedVFRep string
		shouldFail    bool
	}{
		{
			name:   "VF representor found",
			uplink: &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			vfIndex:       2,
			expectedVFRep: "eth2",
			shouldFail:    false,
		},
		{
			name:   "VF representor not found - index doesn't exist",
			uplink: &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			vfIndex:       5,
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:          "VF representor not found - no representors",
			uplink:        &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps:        []*repContext{},
			vfIndex:       0,
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:   "VF representor not found - invalid phys_port_name",
			uplink: &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "invalid", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"}, // SF instead of VF
			},
			vfIndex:       0,
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:   "VF representor not found - missing phys_port_name",
			uplink: &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "", PhysSwitchID: "c2cfc60003a1420c"}, // No phys_port_name
				{Name: "eth1", PhysPortName: "pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
			},
			vfIndex:       0,
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:          "uplink is not switchdev",
			uplink:        &repContext{Name: "eth0", PhysPortName: "", PhysSwitchID: ""},
			vfReps:        []*repContext{},
			vfIndex:       0,
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:   "VF representor found with mixed representors",
			uplink: &repContext{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "invalid", PhysSwitchID: "c2cfc60003a1420c"}, // Invalid
				{Name: "eth1", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"}, // SF rep
				{Name: "eth3", PhysPortName: "pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			vfIndex:       2,
			expectedVFRep: "eth3",
			shouldFail:    false,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			// mock netlink calls, trigger failure to fallback to sysfs
			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			defer netlinkops.ResetNetlinkOps()
			nlOpsMock.On("DevLinkGetDevicePortList", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(
				nil, fmt.Errorf("failed to get devlink ports"))

			teardown := setupRepresentorEnvForGetVfRepresentor(t, tcase.uplink, tcase.vfReps)
			defer teardown()
			vfRep, err := GetVfRepresentor(tcase.uplink.Name, tcase.vfIndex)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedVFRep, vfRep)
			}
		})
	}

	// Test edge case: uplink directory doesn't exist (filesystem error)
	t.Run("uplink directory doesn't exist", func(t *testing.T) {
		teardown := setupFakeFs(t)
		defer teardown()

		_, err := GetVfRepresentor("nonexistent_uplink", 0)
		assert.Error(t, err)
	})
}

func TestGetUplinkRepresentorWithPhysPortName(t *testing.T) {
	tcases := []struct {
		name                 string
		vfPciAddress         string
		uplinkRep            *repContext
		vfReps               []*repContext
		expectedUplinkNetdev string
		shouldFail           bool
	}{
		{
			name:         "uplink representor exists",
			vfPciAddress: "0000:03:00.4",
			uplinkRep:    &repContext{Name: "eth0", PhysPortName: "p0", PhysSwitchID: "111111"},
			vfReps: []*repContext{
				{Name: "enp_0", PhysPortName: "pf0vf0", PhysSwitchID: "111111"},
				{Name: "enp_1", PhysPortName: "pf0vf1", PhysSwitchID: "111111"},
			},
			expectedUplinkNetdev: "eth0",
			shouldFail:           false,
		},
		{
			name:                 "uplink representor exists with PF instead of VF",
			vfPciAddress:         "0000:03:00.0",
			uplinkRep:            &repContext{Name: "eth0", PhysPortName: "p0", PhysSwitchID: "111111"},
			vfReps:               []*repContext{},
			expectedUplinkNetdev: "eth0",
			shouldFail:           false,
		},
		{
			name:         "uplink representor does not exist",
			vfPciAddress: "0000:03:00.4",
			uplinkRep:    &repContext{Name: "eth0", PhysPortName: "", PhysSwitchID: ""},
			vfReps: []*repContext{
				{Name: "enp_0", PhysPortName: "pf0vf0", PhysSwitchID: "111111"},
				{Name: "enp_1", PhysPortName: "pf0vf1", PhysSwitchID: "111111"},
			},
			expectedUplinkNetdev: "",
			shouldFail:           true,
		},
		{
			name:         "uplink representor missing switch id",
			vfPciAddress: "0000:03:00.4",
			uplinkRep:    &repContext{Name: "eth0", PhysPortName: "p0", PhysSwitchID: ""},
			vfReps: []*repContext{
				{Name: "enp_0", PhysPortName: "pf0vf0", PhysSwitchID: "111111"},
				{Name: "enp_1", PhysPortName: "pf0vf1", PhysSwitchID: "111111"},
			},
			expectedUplinkNetdev: "",
			shouldFail:           true,
		},
		{
			name:                 "no representors",
			vfPciAddress:         "0000:03:00.4",
			uplinkRep:            &repContext{Name: "eth0", PhysPortName: "", PhysSwitchID: ""},
			vfReps:               []*repContext{},
			expectedUplinkNetdev: "",
			shouldFail:           true,
		},
		{
			name:                 "missing uplink",
			vfPciAddress:         "0000:03:00.4",
			uplinkRep:            nil,
			vfReps:               []*repContext{},
			expectedUplinkNetdev: "",
			shouldFail:           true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupUplinkRepresentorEnv(t, tcase.uplinkRep, tcase.vfPciAddress, tcase.vfReps)
			defer teardown()

			uplinkNetdev, err := GetUplinkRepresentor(tcase.vfPciAddress)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedUplinkNetdev, uplinkNetdev)
			}
		})
	}
}

func TestGetUplinkRepresentorForPfSuccess(t *testing.T) {
	pfPciAddress := "0000:03:00.0"
	uplinkRep := &repContext{Name: "eth0", PhysPortName: "p0", PhysSwitchID: "111111"}

	vfPciAddress := ""
	var vfsReps []*repContext

	teardown := setupUplinkRepresentorEnv(t, uplinkRep, vfPciAddress, vfsReps)
	_ = utilfs.Fs.MkdirAll(filepath.Join(PciSysDir, pfPciAddress, "net", uplinkRep.Name), os.FileMode(0755))
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(pfPciAddress)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", uplinkNetdev)
}

func TestGetUplinkRepresentorErrorEmptySwID(t *testing.T) {
	var testErr error
	vfPciAddress := "0000:03:00.4"
	uplinkRep := &repContext{Name: "eth0", PhysPortName: "", PhysSwitchID: ""}
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

func TestGetVfRepresentorDPU(t *testing.T) {
	tcases := []struct {
		name          string
		vfReps        []*repContext
		pfID          string
		vfID          string
		expectedVFRep string
		shouldFail    bool
	}{
		{
			name: "Host VFs only",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			vfID:          "2",
			expectedVFRep: "eth2",
			shouldFail:    false,
		},
		{
			name: "Host VFs and DPU VFs",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth3", PhysPortName: "c1pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			vfID:          "2",
			expectedVFRep: "eth3",
			shouldFail:    false,
		},
		{
			name: "Host VFs only - Legacy (rep names dont have controller prefix)",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			vfID:          "2",
			expectedVFRep: "eth2",
			shouldFail:    false,
		},
		{
			name: "VF representor not found",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf0vf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0vf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			vfID:          "5",
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID",
			vfReps:        []*repContext{},
			pfID:          "3",
			vfID:          "5",
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID - 2",
			vfReps:        []*repContext{},
			pfID:          "bla",
			vfID:          "5",
			expectedVFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid vfID",
			vfReps:        []*repContext{},
			pfID:          "0",
			vfID:          "bla",
			expectedVFRep: "",
			shouldFail:    true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", tcase.vfReps)
			defer teardown()
			vfRep, err := GetVfRepresentorDPU(tcase.pfID, tcase.vfID)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedVFRep, vfRep)
			}
		})
	}
}

func TestGetPfRepresentorDPU(t *testing.T) {
	tcases := []struct {
		name          string
		pfReps        []*repContext
		pfID          string
		expectedPfRep string
		shouldFail    bool
	}{
		{
			name: "PF representor with controller index",
			pfReps: []*repContext{
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "p1", PhysPortName: "p1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth0", PhysPortName: "c1pf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf1", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			expectedPfRep: "eth0",
			shouldFail:    false,
		},
		{
			name: "PF representor with controller index - pf1",
			pfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf1", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "1",
			expectedPfRep: "eth1",
			shouldFail:    false,
		},
		{
			name: "PF representor without controller index (legacy)",
			pfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf1", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "1",
			expectedPfRep: "eth1",
			shouldFail:    false,
		},
		{
			name: "PF representor not found",
			pfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "1",
			expectedPfRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID",
			pfReps:        []*repContext{},
			pfID:          "3",
			expectedPfRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID - non-numeric",
			pfReps:        []*repContext{},
			pfID:          "bla",
			expectedPfRep: "",
			shouldFail:    true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", tcase.pfReps)
			defer teardown()
			pfRep, err := GetPfRepresentorDPU(tcase.pfID)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedPfRep, pfRep)
			}
		})
	}
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
		teardown()
		t.Errorf("setupSfRepresentorEnv, got %v", err)
	}
	for _, rep := range sfReps {
		repPath := filepath.Join(pfNetPath, rep.Name)
		repLink := filepath.Join(NetSysDir, rep.Name)

		err = utilfs.Fs.MkdirAll(repPath, os.FileMode(0755))
		if err != nil {
			teardown()
			t.Errorf("setupSfRepresentorEnv, got %v", err)
		}

		_ = utilfs.Fs.Symlink(repPath, repLink)
		if err = setUpRepPhysFiles(rep); err != nil {
			teardown()
			t.Errorf("setupSfRepresentorEnv, got %v", err)
		}
	}

	return teardown
}

func TestGetSfRepresentor(t *testing.T) {
	tcases := []struct {
		name          string
		uplink        string
		sfReps        []*repContext
		sfIndex       int
		expectedSFRep string
		shouldFail    bool
	}{
		{
			name:   "Local SFs only",
			uplink: "p0",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "eth2",
			sfIndex:       2,
			shouldFail:    false,
		},
		{
			name:   "Local SFs and External SFs, should return local SF representor",
			uplink: "p0",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth3", PhysPortName: "pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "eth3",
			sfIndex:       2,
			shouldFail:    false,
		},
		{
			name:   "SF rep no found",
			uplink: "p0",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "",
			sfIndex:       2,
			shouldFail:    true,
		},
		{
			name:          "SF rep no found no reps",
			uplink:        "p0",
			sfReps:        []*repContext{},
			expectedSFRep: "",
			sfIndex:       2,
			shouldFail:    true,
		},
		{
			name:   "SF rep no found only external reps",
			uplink: "p0",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "",
			sfIndex:       2,
			shouldFail:    true,
		},
		{
			name:   "SF rep no found sf index not found",
			uplink: "p0",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "",
			sfIndex:       3,
			shouldFail:    true,
		},
		{
			name:   "SF rep no found no uplink",
			uplink: "p1",
			sfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf3", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf3", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedSFRep: "",
			sfIndex:       3,
			shouldFail:    true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			// mock netlink calls, trigger failure to fallback to sysfs
			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			defer netlinkops.ResetNetlinkOps()
			nlOpsMock.On("DevLinkGetDevicePortList", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(
				nil, fmt.Errorf("failed to get devlink ports"))

			teardown := setupSfRepresentorEnv(t, tcase.sfReps)
			defer teardown()
			sfRep, err := GetSfRepresentor(tcase.uplink, tcase.sfIndex)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedSFRep, sfRep)
			}
		})
	}
}

func TestGetPortIndexFromRepresentor(t *testing.T) {
	tcases := []struct {
		name          string
		netdev        string
		reps          []*repContext
		expectedID    int
		shouldFail    bool
		expectedError string
	}{
		{
			name:   "VF rep",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "pf0vf5", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    5,
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:   "SF rep",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "pf0sf5", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    5,
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:   "external VF rep",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "c1pf0vf5", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    5,
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:   "externalSF rep",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "c1pf0sf5", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    5,
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:   "unsupported pf rep",
			netdev: "pf0hpf",
			reps: []*repContext{
				{Name: "pf0hpf", PhysPortName: "pf0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    0,
			shouldFail:    true,
			expectedError: "unsupported port flavor",
		},
		{
			name:   "unsupported uplink rep",
			netdev: "p0",
			reps: []*repContext{
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    0,
			shouldFail:    true,
			expectedError: "unsupported port flavor",
		},
		{
			name:   "netdev does not have phys_port_name",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedID:    0,
			shouldFail:    true,
			expectedError: "no such file or directory",
		},
		{
			name:   "netdev is not a representor",
			netdev: "eth5",
			reps: []*repContext{
				{Name: "eth5", PhysPortName: "p0", PhysSwitchID: ""},
			},
			expectedID:    0,
			shouldFail:    true,
			expectedError: "does not represent an eswitch port",
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", tcase.reps)
			defer teardown()

			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			defer netlinkops.ResetNetlinkOps()

			nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
				nil, fmt.Errorf("failed to get devlink port"))

			portID, err := GetPortIndexFromRepresentor(tcase.netdev)
			if tcase.shouldFail {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tcase.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, portID, tcase.expectedID)
			}
		})
	}
}

func TestGetSfRepresentorDPU(t *testing.T) {
	tcases := []struct {
		name          string
		vfReps        []*repContext
		pfID          string
		sfID          string
		expectedSFRep string
		shouldFail    bool
	}{
		{
			name: "Host SFs only",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "c1pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "c1pf0sf3", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			sfID:          "2",
			expectedSFRep: "eth1",
			shouldFail:    false,
		},
		{
			name: "Host SFs and DPU SFs",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth3", PhysPortName: "c1pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			sfID:          "2",
			expectedSFRep: "eth3",
			shouldFail:    false,
		},
		{
			name: "DPU SFs only (rep names dont have controller prefix)",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth1", PhysPortName: "pf0sf1", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "eth2", PhysPortName: "pf0sf2", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			sfID:          "2",
			expectedSFRep: "",
			shouldFail:    true,
		},
		{
			name: "SF representor not found",
			vfReps: []*repContext{
				{Name: "eth0", PhysPortName: "c1pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			pfID:          "0",
			sfID:          "5",
			expectedSFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID",
			vfReps:        []*repContext{},
			pfID:          "3",
			sfID:          "5",
			expectedSFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid pfID - 2",
			vfReps:        []*repContext{},
			pfID:          "bla",
			sfID:          "5",
			expectedSFRep: "",
			shouldFail:    true,
		},
		{
			name:          "invalid sfID",
			vfReps:        []*repContext{},
			pfID:          "0",
			sfID:          "bla",
			expectedSFRep: "",
			shouldFail:    true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", tcase.vfReps)
			defer teardown()
			vfRep, err := GetSfRepresentorDPU(tcase.pfID, tcase.sfID)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedSFRep, vfRep)
			}
		})
	}
}

func TestGetVfRepresentorPortFlavour(t *testing.T) {
	tcases := []struct {
		name       string
		netdev     string
		rep        repContext
		expected   PortFlavour
		shouldFail bool
	}{
		{
			name:       "Physical flavor",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PHYSICAL,
			shouldFail: false,
		},
		{
			name:       "PF flavor",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "pf0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PCI_PF,
			shouldFail: false,
		},
		{
			name:       "VF flavor",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PCI_VF,
			shouldFail: false,
		},
		{
			name:       "VF flavor external VF",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "c1pf0vf0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PCI_VF,
			shouldFail: false,
		},
		{
			name:       "SF flavor",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PCI_SF,
			shouldFail: false,
		},
		{
			name:       "SF flavor external SF",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "c1pf0sf0", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_PCI_SF,
			shouldFail: false,
		},
		{
			name:       "unknown flavor - not switchdev",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "pf0vf0", PhysSwitchID: ""},
			expected:   PORT_FLAVOUR_UNKNOWN,
			shouldFail: true,
		},
		{
			name:       "unknown flavor - not phys_port_name",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_UNKNOWN,
			shouldFail: true,
		},
		{
			name:       "unknown flavor - invalid phys_port_name",
			netdev:     "eth0",
			rep:        repContext{Name: "eth0", PhysPortName: "invalid", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_UNKNOWN,
			shouldFail: false,
		},
		{
			name:       "unknown flavor - no device",
			netdev:     "eth1",
			rep:        repContext{Name: "eth0", PhysPortName: "pf0vf34", PhysSwitchID: "c2cfc60003a1420c"},
			expected:   PORT_FLAVOUR_UNKNOWN,
			shouldFail: true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", []*repContext{&tcase.rep})
			defer teardown()

			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			defer netlinkops.ResetNetlinkOps()

			nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
				nil, fmt.Errorf("failed to get devlink port"))

			f, err := GetRepresentorPortFlavour(tcase.netdev)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expected, f)
			}
		})
	}
}

func TestGetVfRepresentorPortFlavourDevlink(t *testing.T) {
	nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
	netlinkops.SetNetlinkOps(nlOpsMock)
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
		{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
		{Name: "pf0hpf", PhysPortName: "pf0", PhysSwitchID: "c2cfc60003a1420c"},
		{Name: "pf0vf5", PhysPortName: "pf0vf5", PhysSwitchID: "c2cfc60003a1420c"},
		{Name: "pf0vf10", PhysPortName: "c1pf0vf10", PhysSwitchID: "c2cfc60003a1420c"},
		{Name: "pf0sf5", PhysPortName: "pf0sf5", PhysSwitchID: "c2cfc60003a1420c"},
		{Name: "pf0sf10", PhysPortName: "c1pf0sf10", PhysSwitchID: "c2cfc60003a1420c"},
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
	setupDPUConfigFileForPort(t, "p0", "pf", repConfigFile)
	// Run test
	tcases := []struct {
		name        string
		netdev      string
		expectedMac string
		shouldFail  bool
	}{
		{name: "PF rep", netdev: "pf0hpf", expectedMac: "0c:42:a1:de:cf:7c", shouldFail: false},
		{name: "VF rep", netdev: "pf0vf5", expectedMac: "", shouldFail: true},
		{name: "Ext VF rep", netdev: "pf0vf10", expectedMac: "", shouldFail: true},
		{name: "SF rep", netdev: "pf0sf5", expectedMac: "", shouldFail: true},
		{name: "Ext SF rep", netdev: "pf0sf10", expectedMac: "", shouldFail: true},
		{name: "Physical rep", netdev: "p0", expectedMac: "", shouldFail: true},
		{name: "Unknown rep", netdev: "foobar", expectedMac: "", shouldFail: true},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(
				nil, fmt.Errorf("failed to get devlink port"))

			mac, err := GetRepresentorPeerMacAddress(tcase.netdev)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedMac, mac.String())
			}
		})
	}
}

func TestGetRepresentorPeerMacAddressDevlink(t *testing.T) {
	nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
	netlinkops.SetNetlinkOps(nlOpsMock)
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
	pfID := "0"
	vfIdx := "24"
	mac := net.HardwareAddr{0, 0, 0, 1, 2, 3}

	tcases := []struct {
		name        string
		netdev      string
		reps        []*repContext
		expectedMac string
		shouldFail  bool
	}{
		{
			name:   "VF rep with external controller",
			netdev: "pf0vf24",
			reps: []*repContext{
				{Name: "pf0vf24", PhysPortName: "c1pf0vf24", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedMac: mac.String(),
			shouldFail:  false,
		},
		{
			name:   "VF rep without external controller - Legacy",
			netdev: "pf0vf24",
			reps: []*repContext{
				{Name: "pf0vf24", PhysPortName: "pf0vf24", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedMac: mac.String(),
			shouldFail:  false,
		},
		{
			name:   "PF rep should fail",
			netdev: "pf0hpf",
			reps: []*repContext{
				{Name: "pf0hpf", PhysPortName: "pf0", PhysSwitchID: "c2cfc60003a1420c"},
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedMac: "",
			shouldFail:  true,
		},
		{
			name:   "non existent representor should fail",
			netdev: "foobar",
			reps: []*repContext{
				{Name: "p0", PhysPortName: "p0", PhysSwitchID: "c2cfc60003a1420c"},
			},
			expectedMac: "",
			shouldFail:  true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			teardown := setupRepresentorEnv(t, "", tcase.reps)
			defer teardown()

			nlOpsMock := netlinkopsMocks.NewMockNetlinkOps(t)
			netlinkops.SetNetlinkOps(nlOpsMock)
			defer netlinkops.ResetNetlinkOps()

			nlOpsMock.On("DevLinkGetPortByNetdevName", mock.AnythingOfType("string")).Return(nil, fmt.Errorf("no devlink support"))

			//  setup sysfs layout
			path := fmt.Sprintf("%s/p%s/smart_nic/vf%s", NetSysDir, pfID, vfIdx)
			_ = utilfs.Fs.MkdirAll(path, os.FileMode(0755))

			macFile := filepath.Join(path, "mac")
			_, err := utilfs.Fs.Create(macFile)
			assert.NoError(t, err)

			// execute test
			err = SetRepresentorPeerMacAddress(tcase.netdev, mac)
			if tcase.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// verify mac file content
				content, err := utilfs.Fs.ReadFile(macFile)
				assert.NoError(t, err)
				assert.Equal(t, mac.String(), string(content))
			}
		})
	}
}
