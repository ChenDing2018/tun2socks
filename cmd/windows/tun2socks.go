package main

import (
	"C"
	"github.com/xjasonlyu/tun2socks/v2/engine"
	"github.com/xjasonlyu/tun2socks/v2/log"
	"go.uber.org/automaxprocs/maxprocs"
	"gopkg.in/yaml.v3"
	"os"
	"os/signal"
	"syscall"
)

//export StartProxy
func StartProxy(config string) {

	data, err := os.ReadFile(config)
	if err != nil {
		log.Fatalf("Failed to read config file '%s': %v", config, err)
	}
	var key = new(engine.Key)
	if err = yaml.Unmarshal(data, key); err != nil {
		log.Fatalf("Failed to unmarshal config file '%s': %v", config, err)
	}
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))

	engine.Insert(key)
	engine.Start()
	defer engine.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

//export StopProxy
func StopProxy() {
	engine.Stop()
}

func main() {
	// Need a main function to make CGO compile package as C shared library
}
