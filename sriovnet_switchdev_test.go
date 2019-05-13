package sriovnet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	utilfs "github.com/Mellanox/sriovnet/pkg/utils/filesystem"
)

func setupUplinkRepresentorEnv(t *testing.T, vfPciAddress string) func(){
	var err error
	utilfs.Fs = utilfs.NewFakeFs()
	path := filepath.Join(PciSysDir, vfPciAddress, "physfn/net", "eth0")
	err = utilfs.Fs.MkdirAll(path, os.FileMode(0755))
	defer func(){if err != nil {
		t.Errorf("setupUplinkRepresentorEnv, got %v", err)}
	}()
	path = filepath.Join(NetSysDir, "eth0")
	err = utilfs.Fs.MkdirAll(path, os.FileMode(0755))
	path = filepath.Join(NetSysDir, "eth1")
	err = utilfs.Fs.MkdirAll(path, os.FileMode(0755))
	swIDFile := filepath.Join(NetSysDir, "eth0", netdevPhysSwitchID)
	swId, err := utilfs.Fs.Create(swIDFile)
	_, err = swId.Write([]byte("111111"))
	return func(){utilfs.Fs.RemoveAll("/")} //nolint
}

func TestGetUplinkRepresentorSuccess(t *testing.T) {
	vfPciAddress := "0000:03:00.4"
	teardown := setupUplinkRepresentorEnv(t, vfPciAddress)
	defer teardown()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", uplinkNetdev)
}

func TestGetUplinkRepresentorErrorMissingSwID(t *testing.T) {
	var testErr error
	vfPciAddress := "0000:03:00.4"
	expectedError := fmt.Sprintf("uplink for %s not found", vfPciAddress)
	teardown := setupUplinkRepresentorEnv(t, vfPciAddress)
	defer teardown()
	swIDFile := filepath.Join(NetSysDir, "eth0", netdevPhysSwitchID)
	testErr = utilfs.Fs.Remove(swIDFile)
	defer func(){if testErr != nil {
		t.Errorf("setupUplinkRepresentorEnv, got %v", testErr)}
	}()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Equal(t, expectedError, err.Error())
}

func TestGetUplinkRepresentorErrorEmptySwID(t *testing.T) {
	var testErr error
	vfPciAddress := "0000:03:00.4"
	expectedError := fmt.Sprintf("uplink for %s not found", vfPciAddress)
	teardown := setupUplinkRepresentorEnv(t, vfPciAddress)
	defer teardown()
	swIDFile := filepath.Join(NetSysDir, "eth0", netdevPhysSwitchID)
	swId, testErr := utilfs.Fs.Create(swIDFile)
	defer func(){if testErr != nil {
		t.Errorf("setupUplinkRepresentorEnv, got %v", testErr)}
	}()
	_, testErr = swId.Write([]byte(""))
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Equal(t, expectedError, err.Error())
}

func TestGetUplinkRepresentorErrorMissingUplink(t *testing.T) {
	var testErr error
	vfPciAddress := "0000:03:00.4"
	teardown := setupUplinkRepresentorEnv(t, vfPciAddress)
	expectedError := fmt.Sprintf("failed to lookup %s", vfPciAddress)
	defer teardown()
	path := filepath.Join(PciSysDir, vfPciAddress)
	testErr = utilfs.Fs.RemoveAll(path)
	defer func(){if testErr != nil {
		t.Errorf("setupUplinkRepresentorEnv, got %v", testErr)}
	}()
	uplinkNetdev, err := GetUplinkRepresentor(vfPciAddress)
	assert.Error(t, err)
	assert.Equal(t, "", uplinkNetdev)
	assert.Contains(t, err.Error(), expectedError)
}
