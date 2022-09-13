package config

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os/exec"
	"testing"
)

func TestParseConfig(t *testing.T) {

	key, _ := registry.OpenKey(registry.CURRENT_USER, "SOFTWARE\\Classes\\Local Settings\\Software\\Microsoft\\Windows\\CurrentVersion\\AppContainer\\Mappings", registry.ALL_ACCESS)
	subNames, _ := key.ReadSubKeyNames(0)
	for _, name := range subNames {
		arg := fmt.Sprintf("checkNetIsolation loopbackExempt -d -p=%s", name)
		fmt.Println(arg)
		command := exec.Command("cmd", "/c", arg)
		command.Run()
	}
}
