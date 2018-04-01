# sriovnet
Go library to configure SRIOV networking devices

Local build and test

You can use go get command:
```
go get github.com/Mellanox/sriovnet.git
```

Example:

```
package main

import (
    "fmt"
    "github.com/Mellanox/sriovnet"
)

func main() {
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
```
