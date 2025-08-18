module github.com/k8snetworkplumbingwg/sriovnet

go 1.23.0

require (
	github.com/google/uuid v1.6.0
	github.com/spf13/afero v1.14.0
	github.com/stretchr/testify v1.10.0
	github.com/vishvananda/netlink v1.3.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/vishvananda/netlink => github.com/adrianchiris/netlink v1.0.1-0.20250818103237-398598b7042c
