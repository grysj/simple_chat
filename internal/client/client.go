package client

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	pkg "grysj/chat/pkg/models/tcp"
	"grysj/chat/pkg/models/udp"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/ipv4"
)

type Client struct {
	TcpConn       net.Conn
	Username      string
	UdpConn       *net.UDPConn
	UdpAddr       *net.UDPAddr
	mu            sync.Mutex
	MulticastConn *ipv4.PacketConn
	Port          int
}

func NewClient() (*Client, error) {
	var newClient Client

	fmt.Print("Enter your username: ")
	reader := bufio.NewReader(os.Stdin)
	username, err := reader.ReadString('\n')
	username = strings.Trim(username, "\n")
	fmt.Println("Your username is:", username)
	if err != nil {
		return &newClient, err
	}
	newClient.Username = username
	return &newClient, nil
}

func (c *Client) ConnectToServerTCP(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(conn)

	err = enc.Encode(pkg.TcpUsernameReq{
		Username: c.Username,
	})

	if err != nil {
		return err
	}
	var res pkg.TcpUsernameRes
	dec := gob.NewDecoder(conn)
	err = dec.Decode(&res)
	if err != nil {
		return err
	}
	if !res.IsAvailable {
		err = fmt.Errorf("Username taken!")
		return err
	}
	c.TcpConn = conn
	c.UdpAddr = &net.UDPAddr{
		IP: net.ParseIP("127.0.0.1"),
	}
	if localAddr, ok := c.TcpConn.LocalAddr().(*net.TCPAddr); ok {

		c.UdpAddr.Port = localAddr.Port
		c.Port = localAddr.Port

		c.UdpConn, err = net.ListenUDP("udp", c.UdpAddr)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		return err
	}

	return nil
}

func (c *Client) HandleIncomingTCP() {
	defer c.TcpConn.Close()
	dec := gob.NewDecoder(c.TcpConn)
	var msg pkg.TcpMessage
	for {
		err := dec.Decode(&msg)
		if err != nil {
			continue
		}
		if msg.UserFrom == c.Username {
			continue
		}
		fmt.Printf("\r%s: %s\n", msg.UserFrom, msg.Message)
		fmt.Print(c.Username + ": ")
	}
}

func (c *Client) HandleOutgoing() {
	defer c.TcpConn.Close()

	enc := gob.NewEncoder(c.TcpConn)
	var msg pkg.TcpMessage
	msg.UserFrom = c.Username
	reader := bufio.NewReader(os.Stdin)

	for {

		var err error

		fmt.Print(c.Username + ": ")
		msg.Message, err = reader.ReadString('\n')
		msg.Message = strings.Trim(msg.Message, "\n")
		if err != nil {
			fmt.Println("Error: " + err.Error())
			continue
		}
		if len(msg.Message) == 0 {
			continue
		}
		if msg.Message == "/U/" {
			c.SendAsciiUDP()
			continue
		}
		if msg.Message == "/M/" {
			c.SendOnMulticast()
			continue
		}
		err = enc.Encode(msg)
		if err != nil {
			fmt.Println("Error: " + err.Error())
			continue
		}
	}
}

func (c *Client) SendAsciiUDP() {

	msg, err := udp.NewUdpMessage("ascii.txt")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	msg.FromUser = c.Username

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&msg); err != nil {
		fmt.Println("Failed to encode with gob: ", err)
		return
	}

	_, err = c.UdpConn.WriteToUDP(buf.Bytes(), &net.UDPAddr{
		Port: 8080,
		IP:   net.ParseIP("127.0.0.1"),
	})
	if err != nil {
		fmt.Println("Failed to write to UDP connection: ", err)
		return
	}

}

func (c *Client) HandleIncomingUDP() {

	defer c.UdpConn.Close()

	buffer := make([]byte, 1024)
	for {
		c.mu.Lock()
		n, _, err := c.UdpConn.ReadFromUDP(buffer)
		c.mu.Unlock()
		if err != nil {

			log.Print(err)
			continue
		}

		data := buffer[:n]
		dec := gob.NewDecoder(bytes.NewReader(data))

		var msg udp.UdpMessage
		if err := dec.Decode(&msg); err != nil {
			log.Printf("Failed to decode gob message: %v", err)
			continue
		}
		fmt.Printf("\r%s:\n", msg.FromUser)
		fmt.Printf("%s\n", msg.Ascii)
		fmt.Print(c.Username + ": ")
	}
}

func (c *Client) HandleIncomingMulticast() {
	groupAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", "224.0.0.1", 8888))
	if err != nil {
		log.Fatal(err)
	}

	listen := &net.UDPAddr{
		IP:   groupAddr.IP,
		Port: 8888,
	}

	conn, err := net.ListenUDP("udp4", listen)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	p := ipv4.NewPacketConn(conn)

	if err := p.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true); err != nil {
		log.Fatal(err)
	}
	eno1, err := net.InterfaceByName("eno1")
	if err != nil {
		log.Fatal(err)
	}
	if err := p.JoinGroup(eno1, groupAddr); err != nil {
		log.Fatal(err)
	}

	buffer := make([]byte, 1024)

	for {

		n, _, _, err := p.ReadFrom(buffer)
		if err != nil {

			log.Print(err)
			continue
		}
		data := buffer[:n]
		dec := gob.NewDecoder(bytes.NewReader(data))
		var msg udp.UdpMessage
		if err := dec.Decode(&msg); err != nil {
			log.Printf("Failed to decode gob message: %v", err)
			continue
		}
		fmt.Printf("\r%s:\n", msg.FromUser)
		fmt.Printf("%s\n", msg.Ascii)
		fmt.Print(c.Username + ": ")
	}
}

func (c *Client) SendOnMulticast() {
	groupAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", "224.0.0.1", 8888))
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp4", nil, groupAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	p := ipv4.NewPacketConn(conn)

	if err := p.SetMulticastTTL(2); err != nil {
		log.Fatal(err)
	}
	msg, err := udp.NewUdpMessage("ascii.txt")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	msg.FromUser = c.Username

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&msg); err != nil {
		fmt.Println("Failed to encode with gob: ", err)
		return
	}
	if _, err := p.WriteTo(buf.Bytes(), nil, groupAddr); err != nil {
		log.Fatal(err)
		return
	}
}
