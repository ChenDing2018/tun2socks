package config

import (
	"fmt"
	"testing"
)

func TestParseConfig(t *testing.T) {
	lines := readLines("config.conf")
	config := formLines(lines)
	fmt.Println(config)

}
