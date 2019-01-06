package aegrid

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"
)

var dialer = websocket.DefaultDialer

type ClientConfig struct {
	Server      string
	Key         string
	LocalServer string
}

func relay(uid etc.UUID, c *ClientConfig) {
	yo.Println("Relaying", uid, "to", c.LocalServer)
	cn, _, err := dialer.Dial(c.Server+"/api/v1/bind/"+uid.String(), nil)
	if err != nil {
		yo.Warn(err)
		return
	}

	conn, err := net.Dial("tcp", c.LocalServer)
	if err != nil {
		yo.Warn(err)
		return
	}

	go func() {
		for {
			_, msg, err := cn.ReadMessage()
			if err != nil {
				conn.Close()
				cn.Close()
				yo.Warn(err)
				return
			}

			_, err = conn.Write(msg)
			if err != nil {
				conn.Close()
				cn.Close()
				yo.Warn("Error in writing to TCP connection", err)
				return
			}
		}
	}()

	for {
		bytes := make([]byte, 65536)
		i, err := conn.Read(bytes)
		if err != nil {
			yo.Warn("Error in reading from TCP connection", err)
			conn.Close()
			cn.Close()
			return
		}

		err = cn.WriteMessage(websocket.BinaryMessage, bytes[:i])
		if err != nil {
			conn.Close()
			cn.Close()
			yo.Warn(err)
			return
		}
	}

}

const maxErrors = 6

func RunClient(c *ClientConfig) {
	errors := 0

	for {
		uri := c.Server + "/api/v1/accept/" + c.Key
		yo.Println("connecting to", uri)
		cn, _, err := dialer.Dial(uri, nil)
		if err != nil {
			if errors != maxErrors {
				errors++
			}

			tout := time.Second * 3 * time.Duration(errors)
			yo.Warnf("%d tries left: Reconnecting in %v\n", maxErrors-errors, tout)
			time.Sleep(tout)
			continue
		} else {
			errors = 0
		}

		for {
			_, msg, err := cn.ReadMessage()
			if err != nil {
				errors++
				break
			}

			uid := etc.FromBytes(msg).ReadUUID()
			go relay(uid, c)
		}
	}
}
