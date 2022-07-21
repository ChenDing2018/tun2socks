package wireguard_route

import (
	"github.com/xjasonlyu/tun2socks/v2/log"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net/netip"
	"strings"
)

var TABLE, _ = winipcfg.GetIPForwardTable2(windows.AF_UNSPEC)

type WinRoute struct {
	InterfaceLUID winipcfg.LUID // 接口的luid
	DestIpPrefix  string
	NextHop       string
	Metric        uint32
}

func NewWinRoute(luid winipcfg.LUID, dest, next string, metric uint32) *WinRoute {

	if !strings.Contains(dest, "/") {
		dest = dest + "/32"
	}
	return &WinRoute{
		InterfaceLUID: luid,
		DestIpPrefix:  dest,
		NextHop:       next,
		Metric:        metric,
	}
}

func GetV4DefaultRoute() *WinRoute {

	// 查找ipv4默认路由
	for _, t := range TABLE {
		destIpPrefix := t.DestinationPrefix.Prefix().String()
		if destIpPrefix == "0.0.0.0/0" {
			return &WinRoute{
				InterfaceLUID: t.InterfaceLUID,
				DestIpPrefix:  destIpPrefix,
				NextHop:       t.NextHop.Addr().String(),
				Metric:        t.Metric,
			}
		}
	}
	return nil
}

func (wr *WinRoute) DisableAutomaticMetricAndSet(metric uint32) error {
	ipInterface, err := wr.InterfaceLUID.IPInterface(windows.AF_INET)
	if err != nil {
		return err
	}
	ipInterface.UseAutomaticMetric = false
	ipInterface.Metric = metric
	return ipInterface.Set()
}

func AddRoutes(luid winipcfg.LUID, wrs []*WinRoute) {

	var rds []*winipcfg.RouteData
	for _, r := range wrs {
		dest, _ := netip.ParsePrefix(r.DestIpPrefix)
		rd := &winipcfg.RouteData{
			Destination: dest,
			NextHop:     netip.MustParseAddr(r.NextHop),
			Metric:      r.Metric,
		}
		rds = append(rds, rd)
	}

	err := luid.AddRoutes(rds)
	if err != nil && err != windows.ERROR_OBJECT_ALREADY_EXISTS {
		log.Fatalf("add routes fail; err:%s", err)
	}
	log.Debugf("add routes success")
}

func (wr *WinRoute) AddRoute() {
	dest, _ := netip.ParsePrefix(wr.DestIpPrefix)
	err := wr.InterfaceLUID.AddRoute(dest, netip.MustParseAddr(wr.NextHop), wr.Metric)
	if err != nil && err != windows.ERROR_OBJECT_ALREADY_EXISTS {
		log.Fatalf("add route fail; err:%s", err)
	}
	log.Debugf("add route success; dst:%s ,next:%s ,metric:%d", wr.DestIpPrefix, wr.NextHop, wr.Metric)
}

func (wr *WinRoute) DeleteRoute() {
	dest, _ := netip.ParsePrefix(wr.DestIpPrefix)
	err := wr.InterfaceLUID.DeleteRoute(dest, netip.MustParseAddr(wr.NextHop))
	if err != nil {
		log.Fatalf("delete route fail; err:%s", err)
	}
	log.Debugf("delete route success; dst:%s ,next:%s", wr.DestIpPrefix, wr.NextHop)
}
