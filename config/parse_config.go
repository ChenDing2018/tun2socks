package config

import (
	"errors"
	"github.com/xjasonlyu/tun2socks/v2/log"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

func ignoreComments(str string) bool {
	space := strings.TrimSpace(str)
	if len(space) == 0 {
		return true
	}
	if strings.HasPrefix(space, "//") || strings.HasPrefix(space, "#") {
		return true
	}
	return false
}

func ParseConfig(path string) (*Config, error) {
	lines := readLines(path)
	return formLines(lines)
}

func readLines(path string) []string {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("read config ini fail, err:%s", err)
	}
	str := string(file)
	lines := strings.Split(str, "\n")

	var restLines []string
	for _, l := range lines {
		trimSpace := strings.TrimSpace(l)
		if len(trimSpace) == 0 {
			continue
		}
		restLines = append(restLines, trimSpace)
	}

	return restLines
}

func formLines(lines []string) (*Config, error) {

	config := Config{}
	// general
	general := General{
		LogLevel:   "debug",
		Mtu:        1500,
		UdpTimeout: 60,
	}
	generalLines := getLinesBySection("general", lines)
	for _, l := range generalLines {
		var parts []string
		for _, l := range strings.Split(l, "=") {
			parts = append(parts, strings.TrimSpace(l))
		}
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "log-level":
			general.LogLevel = parts[1]
		case "device":
			general.Device = parts[1]
		case "mtu":
			general.Mtu, _ = strconv.Atoi(parts[1])
		case "udp-timeout":
			general.UdpTimeout, _ = strconv.Atoi(parts[1])
		}
	}
	if general.Device == "" {
		return nil, errors.New("lack [general] device info")
	}
	config.General = general
	// proxy
	proxyLines := getLinesBySection("proxy", lines)
	if len(proxyLines) >= 1 {
		config.Proxy = proxyLines[0]
	} else {
		return nil, errors.New("lack [proxy] info")
	}
	// tun-routes
	tunRouteLines := getLinesBySection("tun-routes", lines)
	tunRoutes := TunRoutes{}

	for _, line := range tunRouteLines {
		tunRoutes = append(tunRoutes, line)
	}
	config.TunRoutes = tunRoutes

	//physics-routes
	physicsRouteLines := getLinesBySection("physics-routes", lines)
	physicsRoutes := PhysicsRoutes{}

	for _, line := range physicsRouteLines {
		physicsRoutes = append(physicsRoutes, line)
	}
	config.PhysicsRoutes = physicsRoutes

	// dns
	dns := Dns{}
	dnsLines := getLinesBySection("dns", lines)
	for _, line := range dnsLines {
		dns = append(dns, line)
	}
	config.Dns = dns
	return &config, nil
}

func getLinesBySection(section string, lines []string) []string {
	var restRows []string
	currSect := ""
	for _, line := range lines {
		if ignoreComments(line) {
			continue
		}
		sect := getSection(line)
		if sect != "" {
			currSect = sect
			continue
		}
		if currSect == section {
			restRows = append(restRows, strings.TrimSpace(line))
		}
	}
	return restRows
}

func getSection(s string) string {
	reg, _ := regexp.Compile("^\\s*\\[\\s*([^\\]]*)\\s*\\]\\s*$")
	matchs := reg.FindAllStringSubmatch(s, 1)
	if len(matchs) > 0 && len(matchs[0]) > 1 {
		return matchs[0][1]
	}
	return ""
}
