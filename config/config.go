package config

type Config struct {
	General       General
	Proxy         string
	TunRoutes     TunRoutes
	PhysicsRoutes PhysicsRoutes
	Dns           Dns
}

type General struct {
	Device     string
	LogLevel   string
	Mtu        int
	UdpTimeout int
}

type TunRoutes []string
type PhysicsRoutes []string
type Dns []string
