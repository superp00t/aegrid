package main

import (
	"github.com/superp00t/aegrid"
	"github.com/superp00t/etc/yo"
)

func main() {
	yo.Stringf("s", "server", "AEGrid", "ws://localhost:43818")
	yo.Stringf("k", "key", "port mapping key", "")
	yo.Stringf("l", "local", "local TCP server to port map to", "localhost:80")

	yo.Main("client for AEGrid", func(a []string) {
		if yo.StringG("k") == "" {
			yo.Fatal("you need a key (--k, --key) to do this.")
		}

		aegrid.RunClient(&aegrid.ClientConfig{
			Server:      yo.StringG("s"),
			Key:         yo.StringG("k"),
			LocalServer: yo.StringG("l"),
		})
	})

	yo.Init()
}
