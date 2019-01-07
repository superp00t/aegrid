package aegrid

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"
)

type server struct {
	c           *ServerConfig
	connChannel *sync.Map
	connections *sync.Map
	listeners   *sync.Map
}

type ServerConfig struct {
	Listen   string            `toml:"listen"`
	Mappings map[string]string `toml:"mappings"`
}

type connStorage struct {
	tcp   net.Conn
	stamp time.Time
}

func Server(c *ServerConfig) http.Handler {
	s := new(server)
	s.c = c
	s.connChannel = new(sync.Map)
	s.connections = new(sync.Map)
	s.listeners = new(sync.Map)

	for key, mapping := range c.Mappings {
		go func(k, m string) {
			ch := make(chan etc.UUID)
			s.connChannel.Store(k, ch)

			l, err := net.Listen("tcp", m)
			if err != nil {
				yo.Fatal(err)
			}

			yo.Okf("Listening on %s (%s)\n", m, k)

			for {
				tcpconn, err := l.Accept()
				if err != nil {
					yo.Fatal(err)
				}

				tcpconn.SetReadDeadline(time.Now().Add(20 * time.Second))

				uid := etc.GenerateRandomUUID()

				if _, ok := s.listeners.Load(k); !ok {
					yo.Okf("No listeners available for %s\n", k)
					continue
				}

				yo.Okf("(%s) Accepting tcp connection from %s\n", k, tcpconn.RemoteAddr())
				s.connections.Store(uid, connStorage{
					tcpconn,
					time.Now()})

				go func(ch chan etc.UUID, uid etc.UUID) {
					ch <- uid
				}(ch, uid)
			}
		}(key, mapping)
	}

	m := mux.NewRouter()

	v1 := m.PathPrefix("/api/v1/").Subrouter()

	v1.HandleFunc("/accept/{key}", s.provision)
	v1.HandleFunc("/bind/{token}", s.bind)

	return m
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *server) purge(key string, e etc.UUID) {
	if i, ok := s.listeners.Load(key); ok {
		u := i.(etc.UUID)
		if e == u {
			s.listeners.Delete(key)
		}
	}
}

func (s *server) provision(rw http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)

	mp := s.c.Mappings[v["key"]]
	if mp == "" {
		http.Error(rw, "unauthorized", 401)
		return
	}

	_ch, ok := s.connChannel.Load(v["key"])
	if !ok {
		yo.Warn("No conn channel for", v["key"])
		http.Error(rw, "no corresponding connection queue", http.StatusInternalServerError)
		return
	}

	e := etc.GenerateRandomUUID()
	s.listeners.Store(v["key"], e)
	defer s.purge(v["key"], e)

	ch := _ch.(chan etc.UUID)

	yo.Println("Opening from", r.RemoteAddr)

	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	for {
		uid, ok := <-ch
		if !ok {
			conn.Close()
			return
		}

		_cn, ok := s.connections.Load(uid)
		if !ok {
			continue
		}

		cn := _cn.(connStorage)

		// Too old.
		if time.Since(cn.stamp) > 10*time.Second {
			yo.Println("connection too old.")
			continue
		}

		e := etc.NewBuffer()
		e.WriteUUID(uid)

		err := conn.WriteMessage(websocket.BinaryMessage, e.Bytes())
		if err != nil {
			yo.Warn(err)
			return
		}
	}
}

type wsconn struct {
	*websocket.Conn
}

func (w wsconn) Write(b []byte) (int, error) {
	return len(b), w.WriteMessage(websocket.BinaryMessage, b)
}

func (s *server) bind(rw http.ResponseWriter, r *http.Request) {
	_tok := mux.Vars(r)["token"]
	tok, err := etc.ParseUUID(_tok)
	if err != nil {
		return
	}
	_cn, ok := s.connections.Load(tok)
	if !ok {
		http.Error(rw, "no connection", http.StatusBadRequest)
		return
	}

	__cn := _cn.(connStorage)

	_conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	cn := __cn.tcp

	conn := wsconn{_conn}

	go func() {
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				cn.Close()
				s.connections.Delete(tok)
				return
			}

			_, err = cn.Write(b)
			if err != nil {
				yo.Warn(err)
				conn.Close()
				s.connections.Delete(tok)
				return
			}
		}
	}()

	for {
		data := make([]byte, 0xFFFF)
		i, err := cn.Read(data)
		if err != nil {
			yo.Warn(err)
			conn.Close()
			s.connections.Delete(tok)
			return
		}

		buf := data[:i]
		err = conn.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			yo.Warn(err)
			cn.Close()
			s.connections.Delete(tok)
			return
		}
	}
}
