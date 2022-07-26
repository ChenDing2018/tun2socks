package engine

import (
	"github.com/xjasonlyu/tun2socks/v2/config"
	"time"
)

type Key struct {
	MTU                      int           `yaml:"mtu"`
	Mark                     int           `yaml:"fwmark"`
	Proxy                    string        `yaml:"proxy"`
	RestAPI                  string        `yaml:"restapi"`
	Device                   string        `yaml:"device"`
	LogLevel                 string        `yaml:"loglevel"`
	Interface                string        `yaml:"interface"`
	TCPModerateReceiveBuffer bool          `yaml:"tcp-moderate-receive-buffer"`
	TCPSendBufferSize        string        `yaml:"tcp-send-buffer-size"`
	TCPReceiveBufferSize     string        `yaml:"tcp-receive-buffer-size"`
	UDPTimeout               time.Duration `yaml:"udp-timeout"`
	TunRoutes                []string      `yaml:"tunRoutes"`     // 走tun网卡路由信息
	PhysicsRoutes            []string      `yaml:"physicsRoutes"` // 走默认物理网卡路由信息
	Dns                      []string      `yaml:"dns"`
}

func NewConfigKey(config *config.Config) *Key {

	udpTimeout := config.General.UdpTimeout
	ut := time.Duration(udpTimeout) * time.Second
	return &Key{
		MTU:           config.General.Mtu,
		Device:        config.General.Device,
		LogLevel:      config.General.LogLevel,
		Dns:           config.Dns,
		TunRoutes:     config.TunRoutes,
		PhysicsRoutes: config.PhysicsRoutes,
		Proxy:         config.Proxy,
		UDPTimeout:    ut,
	}
}
