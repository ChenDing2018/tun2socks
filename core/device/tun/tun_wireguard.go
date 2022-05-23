//go:build !linux

package tun

import (
	"fmt"
	"golang.org/x/sys/windows"
	"net/netip"

	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/device/iobased"

	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)
var (
	HelloCnWintunRequestedGUID = &windows.GUID{
		Data1: 0xf689d7c9,
		Data2: 0x6f2f,
		Data3: 0x436b,
		Data4: [8]byte{0x8a, 0x53, 0xe5, 0x4f, 0xe3, 0x51, 0xc3, 0x22},
	}

	DnsToSet = []netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("8.8.4.4"),
		netip.MustParseAddr("114.114.114.114"),
	}

	TunIp, _ = netip.ParsePrefix("10.0.0.1/30")
)
type TUN struct {
	*iobased.Endpoint

	nt     *tun.NativeTun
	mtu    uint32
	name   string
	offset int
}

func Open(name string, mtu uint32) (_ device.Device, err error) {
	defer func() {
		if r := recover(); &r != nil {
			err = fmt.Errorf("open tun: %v", r)
		}
	}()

	t := &TUN{name: name, mtu: mtu, offset: offset}

	forcedMTU := defaultMTU
	if t.mtu > 0 {
		forcedMTU = int(t.mtu)
	}

	nt, err := tun.CreateTUNWithRequestedGUID(t.name, HelloCnWintunRequestedGUID, forcedMTU)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	t.nt = nt.(*tun.NativeTun)

	tunMTU, err := nt.MTU()
	if err != nil {
		return nil, fmt.Errorf("get mtu: %w", err)
	}
	t.mtu = uint32(tunMTU)

	ep, err := iobased.New(t, t.mtu, offset)
	if err != nil {
		return nil, fmt.Errorf("create endpoint: %w", err)
	}
	t.Endpoint = ep
	setInterface(t.nt,t.mtu)
	return t, nil
}

func (t *TUN) Read(packet []byte) (int, error) {
	return t.nt.Read(packet, t.offset)
}

func (t *TUN) Write(packet []byte) (int, error) {
	return t.nt.Write(packet, t.offset)
}

func (t *TUN) Name() string {
	name, _ := t.nt.Name()
	return name
}

func (t *TUN) Close() error {
	defer t.Endpoint.Close()
	return t.nt.Close()
}

func setInterface(tun *tun.NativeTun, mtu uint32) error {
	name, err := tun.Name()
	if err != nil {
		return err
	}

	luid := winipcfg.LUID(tun.LUID())

	iface, err := luid.IPInterface(windows.AF_INET)
	if err != nil {
		return fmt.Errorf("failed to get interface: %s", err)
	}
	iface.NLMTU = mtu
	err = iface.Set()
	if err != nil {
		return fmt.Errorf("failed to set MTU: %s", err)
	}
	err = luid.SetIPAddresses([]netip.Prefix{TunIp})
	if err != nil {
		return fmt.Errorf("failed to set local IP on %s interface: %s", name, err)
	}
	err = luid.SetDNS(windows.AF_INET, DnsToSet, nil)
	if err != nil {
		fmt.Errorf("LUID.SetDNS() returned an error: %w", err)
		return nil
	}
	return nil
}
