package sriovnet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
)

type repContext struct {
	Name         string // create files /sys/bus/pci/devices/<vf addr>/physfn/net/<Name> , /sys/class/net/<Name>
	PhysPortName string // conditionally create if string is empty under /sys/class/net/<Name>/phys_port_name
	PhysSwitchID string // conditionally create if string is empty under /sys/class/net/<Name>/phys_switch_id
}

func setUpRepresentorLayout(vfPciAddress string, rep *repContext) error {
	if vfPciAddress != "" {
		path := filepath.Join(PciSysDir, vfPciAddress, "physfn/net", rep.Name)
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
	err = setUpRepresentorLayout(vfPciAddress, uplink)
	if err != nil {
		return nil
	}

	return teardown
}

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

func TestGetVfRepresentorSmartNIC(t *testing.T) {
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

	vfRep, err := GetVfRepresentorSmartNIC("0", "2")
	assert.NoError(t, err)
	assert.Equal(t, "eth2", vfRep)
}

func TestGetVfRepresentorSmartNICNoRep(t *testing.T) {
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

	vfRep, err := GetVfRepresentorSmartNIC("1", "2")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}

func TestGetVfRepresentorSmartNICInvalidPfID(t *testing.T) {
	vfRep, err := GetVfRepresentorSmartNIC("invalid", "2")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}

func TestGetVfRepresentorSmartNICInvalidVfIndex(t *testing.T) {
	vfRep, err := GetVfRepresentorSmartNIC("1", "invalid")
	assert.Error(t, err)
	assert.Equal(t, "", vfRep)
}
