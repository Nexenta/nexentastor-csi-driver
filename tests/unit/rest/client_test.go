package rest_test

import (
	"fmt"

	"github.com/Nexenta/nexentastor-csi-driver/pkg/rest"
)

func ExampleClient_BuildURI() {
	client := &rest.Client{}

	fmt.Println(client.BuildURI("/root", nil))
	fmt.Println(client.BuildURI("/root", map[string]string{"a": "1"}))
	fmt.Println(client.BuildURI("/root", map[string]string{"a": "1", "b": "2"}))

	// Output:
	// /root
	// /root?a=1
	// /root?a=1&b=2
}
