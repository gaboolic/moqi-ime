// Moqi Go 后端主入口
// 参考 python/server.py 实现
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
	moqipb "github.com/gaboolic/moqi-ime/proto"

	// 导入输入法包
	"github.com/gaboolic/moqi-ime/input_methods/fcitx5"
	"github.com/gaboolic/moqi-ime/input_methods/moqi"
	"github.com/gaboolic/moqi-ime/input_methods/rime"
)

// Client 客户端连接
type Client struct {
	ID              string
	GUID            string
	IsWindows8Above bool
	IsMetroApp      bool
	IsUiLess        bool
	IsConsole       bool
	Service         imecore.TextService
}

// ServiceFactory 服务工厂函数
type ServiceFactory func(client *imecore.Client, guid string) imecore.TextService

// Server Moqi 服务器
type Server struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	factories map[string]ServiceFactory // guid -> factory
	reader    *bufio.Reader
	running   bool
}

func stringifyData(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(raw)
}

func logRequestSummary(clientID string, req *imecore.Request) {
	log.Printf(
		"收到请求 client=%s method=%s seq=%d id=%q commandId=%d keyCode=%d charCode=%d repeat=%d scan=%d composing=%q candidates=%d showCandidates=%t cursor=%d data=%s",
		clientID,
		req.Method,
		req.SeqNum,
		req.ID.StringValue(),
		req.ID.IntValue(),
		req.KeyCode,
		req.CharCode,
		req.RepeatCount,
		req.ScanCode,
		req.CompositionString,
		len(req.CandidateList),
		req.ShowCandidates,
		req.CursorPos,
		stringifyData(req.Data),
	)
}

func logResponseSummary(clientID string, resp *imecore.Response) {
	_, err := json.Marshal(resp)
	if err != nil {
		log.Printf("发送响应 client=%s marshal_error=%v", clientID, err)
		return
	}
	//log.Printf("发送响应 client=%s payload=%s", clientID, string(raw))
}

// NewServer 创建服务器
func NewServer() *Server {
	return &Server{
		clients:   make(map[string]*Client),
		factories: make(map[string]ServiceFactory),
		reader:    bufio.NewReader(os.Stdin),
	}
}

// RegisterService 注册输入法服务工厂
func (s *Server) RegisterService(guid string, factory ServiceFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	guid = strings.ToLower(guid)
	s.factories[guid] = factory
	log.Printf("注册输入法服务: %s", guid)
}

// Run 运行服务器
func (s *Server) Run() error {
	s.running = true
	log.Println("Moqi Go 后端服务器已启动")

	for s.running {
		payload, err := readFrame(s.reader)
		if err != nil {
			if err.Error() == "EOF" {
				log.Println("收到 EOF，服务器停止")
				return nil
			}
			return fmt.Errorf("读取错误: %w", err)
		}

		reqMsg, err := decodeClientRequest(payload)
		if err != nil {
			log.Printf("处理消息错误: %v", err)
			continue
		}
		clientID := reqMsg.GetClientId()
		if clientID == "" {
			log.Printf("处理消息错误: 缺少 client_id")
			continue
		}

		if err := s.handleMessage(reqMsg); err != nil {
			log.Printf("处理消息错误: %v", err)
			_ = s.sendResponse(clientID, &imecore.Response{
				SeqNum:  int(reqMsg.GetSeqNum()),
				Success: false,
				Error:   err.Error(),
			})
		}
	}

	return nil
}

func (s *Server) handleMessage(reqMsg *moqipb.ClientRequest) error {
	clientID := reqMsg.GetClientId()
	req := imecore.ParseProtoRequest(reqMsg)

	// logRequestSummary(clientID, &req)

	// 处理请求
	resp := s.handleRequest(clientID, req)

	// 发送响应
	return s.sendResponse(clientID, resp)
}

// handleRequest 处理请求
func (s *Server) handleRequest(clientID string, req *imecore.Request) *imecore.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Method {
	case "init":
		// Moqi 宿主在 init 时通过顶层 id 传递语言配置 GUID。
		// 为了兼容已有调用，也接受 data.guid。
		guid := req.ID.StringValue()
		if guid == "" && req.Data != nil {
			guid, _ = req.Data["guid"].(string)
		}
		guid = strings.ToLower(guid)
		if guid == "" {
			log.Printf("初始化失败 client=%s seq=%d 原因=缺少guid id=%q data=%s", clientID, req.SeqNum, req.ID.StringValue(), stringifyData(req.Data))
			return &imecore.Response{SeqNum: req.SeqNum, Success: false, Error: "缺少 guid"}
		}

		// 创建客户端
		client := &Client{
			ID:              clientID,
			GUID:            guid,
			IsWindows8Above: req.IsWindows8Above,
			IsMetroApp:      req.IsMetroApp,
			IsUiLess:        req.IsUiLess,
			IsConsole:       req.IsConsole,
		}

		// 获取输入法服务工厂
		factory, ok := s.factories[guid]
		if !ok {
			log.Printf("初始化失败 client=%s seq=%d 原因=未知输入法 guid=%s", clientID, req.SeqNum, guid)
			return &imecore.Response{SeqNum: req.SeqNum, Success: false, Error: fmt.Sprintf("未知的输入法: %s", guid)}
		}

		// 创建输入法服务
		moqiClient := &imecore.Client{
			ID:              clientID,
			GUID:            guid,
			IsWindows8Above: req.IsWindows8Above,
			IsMetroApp:      req.IsMetroApp,
			IsUiLess:        req.IsUiLess,
			IsConsole:       req.IsConsole,
		}
		client.Service = factory(moqiClient, guid)
		s.clients[clientID] = client

		// 初始化服务
		initStart := time.Now()
		initOK := client.Service.Init(req)
		log.Printf("初始化服务耗时 client=%s seq=%d guid=%s elapsed=%s success=%t", clientID, req.SeqNum, guid, time.Since(initStart), initOK)
		if !initOK {
			delete(s.clients, clientID)
			log.Printf("初始化失败 client=%s seq=%d guid=%s 原因=Service.Init返回false", clientID, req.SeqNum, guid)
			return &imecore.Response{SeqNum: req.SeqNum, Success: false, Error: "初始化失败"}
		}

		log.Printf("初始化成功 client=%s seq=%d guid=%s windows8=%t metro=%t uiless=%t console=%t", clientID, req.SeqNum, guid, req.IsWindows8Above, req.IsMetroApp, req.IsUiLess, req.IsConsole)

		return &imecore.Response{SeqNum: req.SeqNum, Success: true}

	case "close":
		if client, ok := s.clients[clientID]; ok {
			client.Service.Close()
			delete(s.clients, clientID)
			log.Printf("客户端关闭 client=%s guid=%s", clientID, client.GUID)
		} else {
			log.Printf("客户端关闭 client=%s 但未找到已初始化会话", clientID)
		}
		return &imecore.Response{SeqNum: req.SeqNum, Success: true}

	case "onActivate", "onDeactivate", "filterKeyDown", "onKeyDown",
		"filterKeyUp", "onKeyUp", "onCommand", "onMenu", "onCompositionTerminated",
		"onPreservedKey", "onLangProfileActivated":
		// 转发到输入法服务
		client, ok := s.clients[clientID]
		if !ok {
			log.Printf("请求失败 client=%s seq=%d method=%s 原因=客户端未初始化", clientID, req.SeqNum, req.Method)
			return &imecore.Response{SeqNum: req.SeqNum, Success: false, Error: "客户端未初始化"}
		}

		//log.Printf("转发请求 client=%s seq=%d method=%s guid=%s", clientID, req.SeqNum, req.Method, client.GUID)
		return client.Service.HandleRequest(req)

	default:
		log.Printf("请求失败 client=%s seq=%d method=%s 原因=未知方法", clientID, req.SeqNum, req.Method)
		return &imecore.Response{SeqNum: req.SeqNum, Success: false, Error: fmt.Sprintf("未知的方法: %s", req.Method)}
	}
}

// sendResponse 发送响应
func (s *Server) sendResponse(clientID string, resp *imecore.Response) error {
	logResponseSummary(clientID, resp)
	msg, err := imecore.BuildProtoResponse(clientID, resp)
	if err != nil {
		return fmt.Errorf("构建 protobuf 响应失败: %w", err)
	}
	data, err := encodeServerResponse(msg)
	if err != nil {
		return fmt.Errorf("序列化响应失败: %w", err)
	}
	return writeFrame(os.Stdout, data)
}

// loadInputMethods 加载所有输入法
func loadInputMethods(server *Server) {
	// 获取当前目录
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("获取可执行文件路径失败:", err)
	}
	exeDir := filepath.Dir(exePath)

	// 扫描 input_methods 目录
	inputMethodsDir := filepath.Join(exeDir, "input_methods")
	entries, err := os.ReadDir(inputMethodsDir)
	if err != nil {
		log.Printf("读取 input_methods 目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 读取 ime.json
		imePath := filepath.Join(inputMethodsDir, entry.Name(), "ime.json")
		data, err := os.ReadFile(imePath)
		if err != nil {
			log.Printf("读取 %s 失败: %v", imePath, err)
			continue
		}

		var imeConfig map[string]interface{}
		if err := json.Unmarshal(data, &imeConfig); err != nil {
			log.Printf("解析 %s 失败: %v", imePath, err)
			continue
		}

		guid, _ := imeConfig["guid"].(string)
		name, _ := imeConfig["name"].(string)
		guid = strings.ToLower(guid)
		if guid == "" {
			log.Printf("%s 缺少 guid", entry.Name())
			continue
		}

		log.Printf("加载输入法: %s (%s)", name, guid)

		// 根据输入法名称注册不同的服务实现
		switch entry.Name() {
		case "rime":
			// RIME 输入法
			server.RegisterService(guid, func(client *imecore.Client, g string) imecore.TextService {
				return rime.New(client)
			})
		case "moqi":
			// 拼音输入法
			server.RegisterService(guid, func(client *imecore.Client, g string) imecore.TextService {
				return moqi.New(client)
			})
		case "fcitx5":
			// Fcitx5 输入法
			server.RegisterService(guid, func(client *imecore.Client, g string) imecore.TextService {
				return fcitx5.New(client)
			})
		default:
			// 默认使用拼音输入法
			server.RegisterService(guid, func(client *imecore.Client, g string) imecore.TextService {
				return moqi.New(client)
			})
		}
	}
}

func openLogFile() (*os.File, error) {
	candidates := []string{}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "MoqiIM", "Log", "moqi-ime.log"))
	}
	if tempDir := os.TempDir(); tempDir != "" {
		candidates = append(candidates, filepath.Join(tempDir, "MoqiIM", "Log", "moqi-ime.log"))
	}
	candidates = append(candidates, "moqi-ime.log")

	var lastErr error
	for _, logPath := range candidates {
		logDir := filepath.Dir(logPath)
		if logDir != "." && logDir != "" {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				lastErr = err
				continue
			}
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			return logFile, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func main() {
	// 日志优先写入用户可写目录，避免安装到 Program Files 后启动失败。
	logFile, err := openLogFile()
	if err != nil {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("无法创建日志文件，改用标准错误输出: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Println("=" + strings.Repeat("=", 50))
	log.Println("Moqi Go 后端启动")
	log.Println("=" + strings.Repeat("=", 50))

	// 创建服务器
	server := NewServer()

	// 加载所有输入法
	loadInputMethods(server)

	// 运行服务器
	if err := server.Run(); err != nil {
		log.Fatal("服务器错误:", err)
	}
}
