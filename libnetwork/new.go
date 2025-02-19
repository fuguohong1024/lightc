package libnetwork

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/fuguohong1024/lightc/libnetwork/internal/ipam"
	"github.com/fuguohong1024/lightc/libnetwork/internal/nat"
	"github.com/fuguohong1024/lightc/libnetwork/network"
	"github.com/fuguohong1024/lightc/paths"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/xerrors"
)

func NewNetwork(name string, subnet net.IPNet) (*network.Network, error) {
	return newNetwork(name, subnet, nil)
}

func newNetwork(name string, subnet net.IPNet, nw *network.Network) (*network.Network, error) {
	f, err := os.Open(paths.NetworkLock)
	if err != nil {
		return nil, xerrors.Errorf("open lock file failed: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, xerrors.Errorf("lock network failed: %w", err)
	}
	defer func() {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			logrus.Error(xerrors.Errorf("unlock network failed: %w", err))
		}
	}()

	if nw == nil {
		if _, err := os.Stat(filepath.Join(paths.BridgePath, name)); err == nil {
			return nil, xerrors.Errorf("network %s exists", name)
		}

		file, err := os.OpenFile(filepath.Join(paths.BridgePath, name), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, xerrors.Errorf("create network file failed: %w", err)
		}
		defer func() {
			_ = file.Close()
		}()

		ip, exist, err := ipam.IPAllAllocator.AllocateSubnet(subnet)
		if err != nil {
			return nil, xerrors.Errorf("allocate subnet failed: %w", err)
		}

		if exist {
			_ = os.Remove(file.Name())
			return nil, xerrors.Errorf("subnet %s exists", subnet)
		}

		nw = &network.Network{
			Name:    name,
			Subnet:  subnet,
			Gateway: ip,
		}

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(nw); err != nil {
			return nil, xerrors.Errorf("encode network file failed: %w", err)
		}
	}

	if err := initNetwork(nw); err != nil {
		return nil, xerrors.Errorf("init network failed: %w", err)
	}

	return nw, nil
}

func initNetwork(nw *network.Network) error {
	br, err := createBridge(nw.Name)
	if err != nil {
		return xerrors.Errorf("create bridge failed: %w", err)
	}

	gatewayIP := nw.Subnet
	gatewayIP.IP = nw.Gateway

	if err := setInterfaceIP(br, gatewayIP); err != nil {
		return xerrors.Errorf("set interface gateway IP failed: %w", err)
	}

	if err := setInterfaceUP(br); err != nil {
		return xerrors.Errorf("set bridge up failed: %w", err)
	}

	if err := nat.SetSNAT(nw.Name, nw.Subnet); err != nil {
		return xerrors.Errorf("setup iptables failed: %w", err)
	}

	return nil
}

func createBridge(bridgeName string) (br netlink.Link, err error) {
	if _, err := net.InterfaceByName(bridgeName); err == nil {
		return nil, nil
	}

	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = bridgeName

	br = &netlink.Bridge{LinkAttrs: linkAttrs}

	if err := netlink.LinkAdd(br); err != nil {
		return nil, xerrors.Errorf("add br %s failed: %w", bridgeName, err)
	}

	return br, nil
}

func setInterfaceIP(iface netlink.Link, ipNet net.IPNet) error {
	if err := netlink.AddrAdd(iface, &netlink.Addr{
		IPNet: &ipNet,
	}); err != nil {
		return xerrors.Errorf("add addr to iface failed: %w", err)
	}
	return nil
}

func setInterfaceUP(iface netlink.Link) error {
	if err := netlink.LinkSetUp(iface); err != nil {
		return xerrors.Errorf("set bridge up failed: %w", err)
	}

	return nil
}
