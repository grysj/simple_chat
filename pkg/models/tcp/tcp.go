package tcp

type TcpUsernameReq struct {
	Username string
}

type TcpUsernameRes struct {
	IsAvailable bool
}

type TcpMessage struct {
	UserFrom string
	Message  string
}
