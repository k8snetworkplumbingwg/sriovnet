package sriovnet

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	netSysDir        = "/sys/class/net"
	pcidevPrefix     = "device"
	netdevDriverDir  = "device/driver"
	netdevUnbindFile = "unbind"
	netdevBindFile   = "bind"

	netDevMaxVfCountFile     = "sriov_totalvfs"
	netDevCurrentVfCountFile = "sriov_numvfs"
	netDevVfDevicePrefix     = "virtfn"
)

type VfObject struct {
	NetdevName string
	PCIDevName string
}

func netDevDeviceDir(netDevName string) string {
	devDirName := netSysDir + "/" + netDevName + "/" + pcidevPrefix
	return devDirName
}

func getMaxVfCount(pfNetdevName string) (int, error) {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevMaxVfCountFile,
	}

	maxVfs, err := maxDevFile.ReadInt()
	if err != nil {
		return 0, err
	} else {
		fmt.Println("max_vfs = ", maxVfs)
		return maxVfs, nil
	}
}

func setMaxVfCount(pfNetdevName string, maxVfs int) error {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevCurrentVfCountFile,
	}

	return maxDevFile.WriteInt(maxVfs)
}

func netdevGetEnabledVfCount(pfNetdevName string) (int, error) {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevCurrentVfCountFile,
	}

	curVfs, err := maxDevFile.ReadInt()
	if err != nil {
		return 0, err
	} else {
		fmt.Println("cur_vfs = ", curVfs)
		return curVfs, nil
	}
}

func vfNetdevNameFromParent(pfNetdevName string, vfDir string) string {

	devDirName := netDevDeviceDir(pfNetdevName)

	vfNetdev, _ := lsFilesWithPrefix(devDirName+"/"+vfDir+"/"+"net", "", false)
	if len(vfNetdev) <= 0 {
		return ""
	} else {
		return vfNetdev[0]
	}
}

func vfPCIDevNameFromVfDir(pfNetdevName string, vfDir string) string {
	link := filepath.Join(netSysDir, pfNetdevName, pcidevPrefix, vfDir)
	pciDevDir, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	if len(pciDevDir) <= 3 {
		return ""
	}

	return pciDevDir[3:len(pciDevDir)]
}

func getVfPciDevList(pfNetdevName string) ([]string, error) {
	var vfDirList []string
	var i int
	devDirName := netDevDeviceDir(pfNetdevName)

	virtFnDirs, err := lsFilesWithPrefix(devDirName, netDevVfDevicePrefix, true)

	if err != nil {
		return nil, err
	}

	i = 0
	for _, vfDir := range virtFnDirs {
		vfDirList = append(vfDirList, vfDir)
		i++
	}
	return vfDirList, nil
}

func findVfDirForNetdev(pfNetdevName string, vfNetdevName string) (string, error) {

	virtFnDirs, err := getVfPciDevList(pfNetdevName)
	if err != nil {
		return "", err
	}

	ndevSearchName := vfNetdevName + "__"

	for _, vfDir := range virtFnDirs {

		vfNetdevPath := filepath.Join(netSysDir, pfNetdevName,
			pcidevPrefix, vfDir, "net")
		vfNetdevList, err := lsDirs(vfNetdevPath)
		if err != nil {
			return "", err
		}
		for _, vfName := range vfNetdevList {
			vfNamePrefixed := vfName + "__"
			if ndevSearchName == vfNamePrefixed {
				return vfDir, nil
			}
		}
	}
	return "", fmt.Errorf("device %s not found", vfNetdevName)
}
