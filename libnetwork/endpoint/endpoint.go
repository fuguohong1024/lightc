package endpoint

import (
	"net"

	"github.com/fuguohong1024/lightc/libnetwork/network"
	"github.com/vishvananda/netlink"
)

type Endpoint struct {
	ID      string
	Device  *netlink.Veth
	IP      net.IP
	PortMap []string
	Network *network.Network
}
