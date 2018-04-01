package sriovnet

import (
	"fmt"
	"testing"
)

func TestEnableSRIOV(t *testing.T) {

	err := EnableSRIOV("ib0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisableSRIOV(t *testing.T) {
	err := DisableSRIOV("ib0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetPfHandle(t *testing.T) {
	err1 := EnableSRIOV("ib0")
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

func TestConfigVFs(t *testing.T) {
	err1 := EnableSRIOV("ib0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ib0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVFs(handle)
	if err3 != nil {
		t.Fatal(err3)
	}
	for _, vf := range handle.List {
		fmt.Printf("after config vf = %v\n", vf)
	}
}

func TestAllocFreeVF(t *testing.T) {
	var vfList[10] *VfObj

	err1 := EnableSRIOV("ib0")
	if err1 != nil {
		t.Fatal(err1)
	}

	handle, err2 := GetPfNetdevHandle("ib0")
	if err2 != nil {
		t.Fatal(err2)
	}
	err3 := ConfigVFs(handle)
	if err3 != nil {
		t.Fatal(err3)
	}
	for i := 0; i < 10; i++ {
		vfList[i], _ = AllocateVF(handle)
	}
	for _, vf := range handle.List {
		fmt.Printf("after allocation vf = %v\n", vf)
	}
	for i := 0; i < 10; i++ {
		if vfList[i] == nil {
			continue
		}
		FreeVF(handle, vfList[i])
	}
	for _, vf := range handle.List {
		fmt.Printf("after free vf = %v\n", vf)
	}
}