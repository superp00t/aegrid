package main

import (
	"net/http"

	"github.com/BurntSushi/toml"
	"github.com/superp00t/aegrid"
	"github.com/superp00t/etc/yo"
)

func main() {
	yo.AddSubroutine("run", []string{"config"}, "aegrid server", func(a []string) {
		if len(a) == 0 {
			yo.Fatal("no config")
		}

		var ss aegrid.ServerConfig

		_, err := toml.DecodeFile(a[0], &ss)
		if err != nil {
			yo.Fatal(err)
		}

		yo.Fatal(http.ListenAndServe(ss.Listen, aegrid.Server(&ss)))
	})

	yo.Init()
}
