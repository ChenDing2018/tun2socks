package config

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestParseConfig(t *testing.T) {

	go func() {
		os.Exit(1)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh

	fmt.Println("aaaa")
}
