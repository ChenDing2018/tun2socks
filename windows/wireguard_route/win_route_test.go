package wireguard_route

import (
	"fmt"
	"testing"
)

func TestGetRoute(t *testing.T) {

	route := GetV4DefaultRoute()
	fmt.Println(route)

	var u uint32 = 32
	fmt.Println(u - 20)
}
