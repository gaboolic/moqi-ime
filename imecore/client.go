package imecore

// Client 表示一个 Moqi IME 后端客户端会话。
type Client struct {
	ID              string
	GUID            string
	IsWindows8Above bool
	IsMetroApp      bool
	IsUiLess        bool
	IsConsole       bool
	Service         TextService
}

// TextService 定义输入法服务需要实现的最小接口。
type TextService interface {
	Init(req *Request) bool
	HandleRequest(req *Request) *Response
	Close()
}
