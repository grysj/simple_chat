package server

import (
	"encoding/gob"
	"fmt"
	tcp "grysj/chat/pkg/models/tcp"
	udp "grysj/chat/pkg/models/udp"
	"net"
	"sync"
)

type Server struct {
	mu          sync.Mutex
	clients     map[string]net.Conn
	msgQueueTCP chan tcp.TcpMessage
	l           net.Listener
	clientsUdp  map[string]net.Conn
	msgQueueUDP chan udp.UdpMessage
}

func NewServer(address string) *Server {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return nil
	}
	fmt.Println(listener.Addr())
	server := &Server{
		clients:     make(map[string]net.Conn),
		msgQueueTCP: make(chan tcp.TcpMessage, 10),
		l:           listener,
	}
	go server.handleBroadcast()
	return server
}

func (s *Server) Start() {
	fmt.Println("Server started on", s.l.Addr())
	go s.handleUDP()
	for {
		conn, err := s.l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go s.handleClientTCP(conn)

	}
}

func (s *Server) handleClientTCP(conn net.Conn) {
	var msg tcp.TcpUsernameReq
	dec := gob.NewDecoder(conn)

	err := dec.Decode(&msg)
	if err != nil {
		fmt.Println("Error decoding username request:", err)
		conn.Close()
		return
	}

	s.mu.Lock()
	_, exists := s.clients[msg.Username]
	s.mu.Unlock()

	enc := gob.NewEncoder(conn)
	if exists {
		fmt.Println("Username already exists")
		enc.Encode(tcp.TcpUsernameRes{IsAvailable: false})
		conn.Close()
		return
	}

	s.mu.Lock()
	s.clients[msg.Username] = conn
	s.mu.Unlock()

	enc.Encode(tcp.TcpUsernameRes{IsAvailable: true})

	joinMsg := tcp.TcpMessage{UserFrom: "Server", Message: msg.Username + " joined the chat!"}
	s.msgQueueTCP <- joinMsg

	for {
		var chatMsg tcp.TcpMessage
		err := dec.Decode(&chatMsg)
		if err != nil {
			fmt.Println("Client disconnected:", msg.Username)
			break
		}
		s.msgQueueTCP <- chatMsg
	}

	s.mu.Lock()
	delete(s.clients, msg.Username)
	s.mu.Unlock()
	conn.Close()

	leaveMsg := tcp.TcpMessage{UserFrom: "Server", Message: msg.Username + " left the chat!"}
	s.msgQueueTCP <- leaveMsg
}

func (s *Server) handleBroadcast() {
	for msg := range s.msgQueueTCP {
		s.mu.Lock()
		for _, conn := range s.clients {
			encoder := gob.NewEncoder(conn)
			encoder.Encode(msg)
		}
		s.mu.Unlock()
	}
}

func (s *Server) handleUDP() {

	addr := net.UDPAddr{
		Port: 8080,
		IP:   net.ParseIP("127.0.0.1"),
	}

	connUDP, err := net.ListenUDP("udp", &addr)
	if err != nil {
		panic(err)
	}
	defer connUDP.Close()
	for {
		buffer := make([]byte, 1024)
		_, _, err := connUDP.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		s.mu.Lock()
		for _, conn := range s.clients {
			var port int
			if localAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
				port = localAddr.Port
			} else {
				fmt.Println("error: ", err)
				continue
			}

			_, err = connUDP.WriteToUDP(buffer, &net.UDPAddr{
				Port: port,
				IP:   net.ParseIP("127.0.0.1"),
			})
			if err != nil {
				fmt.Println("Failed to write to UDP connection: ", err)
				continue
			}
		}
		s.mu.Unlock()

	}

}
