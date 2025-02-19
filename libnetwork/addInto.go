package libnetwork

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/fuguohong1024/lightc/info"
	"github.com/fuguohong1024/lightc/libnetwork/endpoint"
	"github.com/fuguohong1024/lightc/libnetwork/internal/ipam"
	"github.com/fuguohong1024/lightc/libnetwork/internal/nat"
	"github.com/fuguohong1024/lightc/paths"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/xerrors"
)

func AddContainerIntoNetwork(networkName string, cInfo *info.Info) error {
	f, err := os.Open(paths.NetworkLock)
	if err != nil {
		return xerrors.Errorf("open lock file failed: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return xerrors.Errorf("lock network failed: %w", err)
	}
	defer func() {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			logrus.Error(xerrors.Errorf("unlock network failed: %w", err))
		}
	}()

	nw, err := loadNetwork(networkName)
	if err != nil {
		return xerrors.Errorf("load network failed: %w", err)
	}

	if _, _, err := net.ParseCIDR(nw.Subnet.String()); err != nil {
		return xerrors.Errorf("network subnet invalid: %w", err)
	}

	ip, err := ipam.IPAllAllocator.Allocate(nw.Subnet)
	if err != nil {
		return xerrors.Errorf("allocate ip failed: %w", err)
	}

	ep := &endpoint.Endpoint{
		ID:      fmt.Sprintf("%s-%s", networkName, cInfo.ID),
		IP:      ip,
		Network: nw,
		PortMap: cInfo.PortMap,
	}

	if len(ep.ID) > 13 {
		ep.ID = ep.ID[:13]
	}

	br, err := netlink.LinkByName(nw.Name)
	if err != nil {
		return xerrors.Errorf("get bridge failed: %w", err)
	}

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = ep.ID
	linkAttrs.MasterIndex = br.Attrs().Index

	ep.Device = &netlink.Veth{
		LinkAttrs: linkAttrs,
		PeerName:  "peer-" + ep.ID,
	}

	if len(ep.Device.PeerName) > 13 {
		ep.Device.PeerName = ep.Device.PeerName[:13]
	}

	if err := netlink.LinkAdd(ep.Device); err != nil {
		return xerrors.Errorf("add endpoint device failed: %w", err)
	}

	if err := netlink.LinkSetUp(ep.Device); err != nil {
		return xerrors.Errorf("set up endpoint device failed: %w", err)
	}

	if err := setEndpointIPAndRoute(ep, cInfo); err != nil {
		return xerrors.Errorf("set endpoint ip and route failed: %w", err)
	}

	nat.SetPortMap(ep)

	cInfo.Network = networkName
	cInfo.IPNet = nw.Subnet
	cInfo.IPNet.IP = ip

	containerHosts, err := os.OpenFile(cInfo.RootFS.Hosts, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return xerrors.Errorf("open hosts file failed: %w", err)
	}
	defer func() {
		_ = containerHosts.Close()
	}()

	if _, err := fmt.Fprintf(containerHosts, "%s	%s\n", cInfo.IPNet.IP.String(), cInfo.ID); err != nil {
		return xerrors.Errorf("write hosts file failed: %w", err)
	}

	return nil
}

func setEndpointIPAndRoute(ep *endpoint.Endpoint, containerInfo *info.Info) error {
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return xerrors.Errorf("get peer name failed: %w", err)
	}

	exitNetns, err := enterNetns(peerLink, containerInfo)
	if err != nil {
		return xerrors.Errorf("enter netns failed: %w", err)
	}
	defer exitNetns()

	interfaceIP := ep.Network.Subnet
	interfaceIP.IP = ep.IP

	if err := setInterfaceIP(peerLink, interfaceIP); err != nil {
		return xerrors.Errorf("set container interface IP failed: %w", err)
	}

	if err := setInterfaceUP(peerLink); err != nil {
		return xerrors.Errorf("set up container interface failed: %w", err)
	}

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return xerrors.Errorf("get iface lo failed: %w", err)
	}

	if err := setInterfaceUP(lo); err != nil {
		return xerrors.Errorf("set up container lo failed: %w", err)
	}

	_, ipNet, _ := net.ParseCIDR("0.0.0.0/0")

	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.Gateway,
		Dst:       ipNet,
	}

	if err := netlink.RouteAdd(defaultRoute); err != nil {
		return xerrors.Errorf("add default route failed: %w", err)
	}

	return nil
}
