//go:build !linux

package tun

import (
	"fmt"
	"github.com/xjasonlyu/tun2socks/v2/windows/wireguard_route"
	"golang.org/x/sys/windows"
	"net/netip"

	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/device/iobased"

	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

const (
	wintunIp   = "10.23.0.2/30"
	wintunGw   = "10.23.0.1"
	defRouteIp = "0.0.0.0/0"
	winTun     = "wintun.dll"
	winTunSite = "https://www.wintun.net/"
)

var (
	WintunRequestedGUID = &windows.GUID{
		Data1: 0xf689d7c9,
		Data2: 0x6f2f,
		Data3: 0x436b,
		Data4: [8]byte{0x8a, 0x53, 0xe5, 0x4f, 0xe3, 0x51, 0xc3, 0x22},
	}

	//DefaultDnsToSet = []netip.Addr{
	//	netip.MustParseAddr("114.114.114.114"),
	//	netip.MustParseAddr("223.5.5.5"),
	//}
	defDns   = []string{"114.114.114.114", "223.5.5.5"}
	TunIp, _ = netip.ParsePrefix(wintunIp)
)

type TUN struct {
	*iobased.Endpoint

	nt              *tun.NativeTun
	mtu             uint32
	name            string
	offset          int
	tunRoutes       []*wireguard_route.WinRoute
	physicsRoutes   []*wireguard_route.WinRoute
	dns             []netip.Addr
	physicsDefRoute *wireguard_route.WinRoute
}

func init() {
	err := windows.NewLazyDLL(winTun).Load()
	if err != nil {
		fmt.Errorf("the %s was not found, you can download it from %s", winTun, winTunSite)
		return
	}
}
func Open(name string, mtu uint32, proxyIp string, dns, tRoutes, pRoutes []string) (_ device.Device, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("open tun: %v", r)
		}
	}()

	forcedMTU := defaultMTU
	if mtu > 0 {
		forcedMTU = int(mtu)
	}

	nt, err := tun.CreateTUN(name, forcedMTU)
	//nt, err := tun.CreateTUNWithRequestedGUID(name, WintunRequestedGUID, forcedMTU)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}

	var nat = nt.(*tun.NativeTun)
	// 默认路由
	defRoute := wireguard_route.GetV4DefaultRoute()
	if defRoute == nil {
		return nil, fmt.Errorf("get default route fail")
	}
	// 设置默认路由对应网卡的metric
	err = defRoute.DisableAutomaticMetricAndSet(45)
	if err != nil {
		return nil, err
	}
	var tunRoutes, physicsRoutes []*wireguard_route.WinRoute

	// dns
	var dnsSet []netip.Addr
	if len(dns) == 0 {
		dns = defDns
	}
	for _, d := range dns {
		dnsSet = append(dnsSet, netip.MustParseAddr(d))
		physicsRoutes = append(physicsRoutes, wireguard_route.NewWinRoute(defRoute.InterfaceLUID, d, defRoute.NextHop, 1))
	}
	// 走默认路由对应的物理网卡
	physicsRoutes = append(physicsRoutes, wireguard_route.NewWinRoute(defRoute.InterfaceLUID, proxyIp, defRoute.NextHop, 1))
	if len(pRoutes) != 0 {
		for _, r := range pRoutes {
			physicsRoutes = append(physicsRoutes, wireguard_route.NewWinRoute(defRoute.InterfaceLUID, r, defRoute.NextHop, 1))
		}
	}
	if len(tRoutes) == 0 {
		route := wireguard_route.NewWinRoute(winipcfg.LUID(nat.LUID()), defRouteIp, wintunGw, 1)
		tunRoutes = append(tunRoutes, route)
	} else {
		for _, r := range tRoutes {
			tunRoutes = append(tunRoutes, wireguard_route.NewWinRoute(winipcfg.LUID(nat.LUID()), r, wintunGw, 1))
		}
	}

	t := &TUN{name: name, mtu: mtu, offset: offset, nt: nat, dns: dnsSet, tunRoutes: tunRoutes, physicsRoutes: physicsRoutes, physicsDefRoute: defRoute}
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

	err = t.setInterface()
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TUN) setInterface() error {
	wintun := t.nt
	name, err := wintun.Name()
	if err != nil {
		return err
	}

	luid := winipcfg.LUID(wintun.LUID())

	iface, err := luid.IPInterface(windows.AF_INET)
	if err != nil {
		return fmt.Errorf("failed to get interface: %s", err)
	}
	// 关闭自动跃点
	iface.Metric = 0
	iface.UseAutomaticMetric = false

	iface.NLMTU = t.mtu
	err = iface.Set()
	if err != nil {
		return fmt.Errorf("failed to set MTU: %s", err)
	}
	fmt.Println(TunIp.String())
	err = luid.SetIPAddresses([]netip.Prefix{TunIp})
	if err != nil {
		return fmt.Errorf("failed to set local IP on %s interface: %s", name, err)
	}
	err = luid.SetDNS(windows.AF_INET, t.dns, nil)
	if err != nil {
		fmt.Errorf("LUID.SetDNS() returned an error: %w", err)
		return nil
	}
	return nil
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
	t.DelRoute()
	return t.nt.Close()
}

func (t *TUN) AddRoute() {
	luid := winipcfg.LUID(t.nt.LUID())
	// add tun route
	wireguard_route.AddRoutes(luid, t.tunRoutes)
	// add proxy route(代理走本地网卡路由)
	wireguard_route.AddRoutes(t.physicsDefRoute.InterfaceLUID, t.physicsRoutes)
}

func (t *TUN) DelRoute() {

	for _, r := range t.tunRoutes {
		r.DeleteRoute()
	}

	for _, r := range t.physicsRoutes {
		r.DeleteRoute()
	}
}
