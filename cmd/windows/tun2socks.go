package main

import (
	"C"
	"github.com/xjasonlyu/tun2socks/v2/config"
	"github.com/xjasonlyu/tun2socks/v2/engine"
	"github.com/xjasonlyu/tun2socks/v2/log"
	"go.uber.org/automaxprocs/maxprocs"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

var (
	runing = false
	ch     = make(chan bool)
)

//export tun2socksStart
func tun2socksStart(configPath *C.char) bool {

	if runing {
		return true
	}
	var configFile = C.GoString(configPath)
	go run(configFile)
	rest := <-ch
	return rest
}

func run(configFile string) {
	var key = &engine.Key{}
	// yml 文件解析
	if configFile != "" && strings.HasSuffix(configFile, ".yml") {
		data, err := os.ReadFile(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file '%s': %v", configFile, err)
		}
		if err = yaml.Unmarshal(data, key); err != nil {
			log.Fatalf("Failed to unmarshal config file '%s': %v", configFile, err)
		}
	}

	if configFile != "" && !strings.HasSuffix(configFile, ".yml") {
		parseConfig, err := config.ParseConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file '%s': %v", configFile, err)
		}
		key = engine.NewConfigKey(parseConfig)
	}
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))

	engine.Insert(key)
	err := engine.Tun2socksStart()
	defer engine.Stop()
	if err != nil {
		ch <- false
	}
	runing = true
	ch <- true
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

//export tun2socksStop
func tun2socksStop() bool {
	err := engine.Tun2socksStop()
	if err != nil {
		return false
	}
	runing = false

	return true
}

//export unrestrictedUwp
func unrestrictedUwp() bool {
	powershell := exec.Command("powershell", "foreach ($n in (get-appxpackage).packagefamilyname) {checknetisolation loopbackexempt -a -n=\"$n\"}") // cmd := exec.Command("powershell")
	err := powershell.Run()
	if err != nil {
		return false
	}
	return true
}

//export restUwp
func restUwp() bool {
	powershell := exec.Command("powershell", "foreach ($n in (get-appxpackage).packagefamilyname) {checknetisolation loopbackexempt -d -n=\"$n\"}") // cmd := exec.Command("powershell")
	err := powershell.Run()
	if err != nil {
		return false
	}
	return true
}
func main() {
	// Need a main function to make CGO compile package as C shared library
}

//  go build -buildmode=c-shared -o tun2socks.dll .\cmd\windows\tun2socks.go
