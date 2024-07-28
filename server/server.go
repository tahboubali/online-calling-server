package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"log"
	"net"
	"net/http"
	"online-calling/debug"
	"os"
	"sync"
)

const (
	CreateUser        = "create-user"
	DeleteUser        = "delete-user"
	UpdateUser        = "update-user"
	CallUpdate        = "call-update"
	GetUsers          = "get-users"
	Success           = "success"
	Error             = "error"
	SuccessCode       = 200
	InternalErrorCode = 500
	BadRequestError   = 400
)

type Server struct {
	*debug.Debugger
	websocket.Upgrader
	Port  string
	Conns map[string]*Conn
	Users map[string]*User
	wg    sync.WaitGroup
	mu    sync.Mutex
}

func NewServer() *Server {
	return &Server{
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		Conns: make(map[string]*Conn),
		Users: make(map[string]*User),
	}
}

type Data struct {
	RequestType string   `json:"request_type"`
	UserInfo    UserInfo `json:"user_info"`
	CallData    CallData `json:"call_data"`
	Username    string   `json:"username"`
}

type CallData struct {
	CurrFrame   []byte `json:"curr_frame"`
	ImageFormat string `json:"image_format"`
}

type Message struct {
	From string
	Data Data
}

func NewMessage(from string, data Data) Message {
	return Message{
		From: from,
		Data: data,
	}
}

type UserInfo struct {
	Username string `json:"username"`
}

type User struct {
	UserInfo
	CurrConn *Conn `json:"-"`
}

func NewUser(userInfo UserInfo) *User {
	return &User{
		UserInfo: userInfo,
	}
}

type Conn struct {
	*websocket.Conn
	CurrUser *User
}

func NewConn(conn *websocket.Conn) *Conn {
	return &Conn{
		Conn: conn,
	}
}

func (c *Conn) sendErr(errCode int, msg string) {
	_ = c.WriteJSON(map[string]any{
		"response_type": Error,
		"code":          errCode,
		"message":       msg,
	})
}

func (c *Conn) sendSuccess(msg string) {
	_ = c.WriteJSON(map[string]any{
		"response_type": Success,
		"code":          SuccessCode,
		"message":       msg,
	})
}

func (s *Server) Init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("error loading .env file: %v", err)
	}
	debugEnv := os.Getenv("DEBUG") == "true"
	if debugEnv {
		log.Println("DEBUG mode is enabled (logs will be printed)")
	} else {
		log.Println("DEBUG mode is disabled (logs will not be printed)")
	}
	s.Debugger = debug.NewDebugger(debugEnv)
	s.Port = os.Getenv("PORT")
}

func (s *Server) Run() {
	s.Init()
	http.HandleFunc("/ws", s.handleWsConn)
	log.Println("server started on localhost" + s.Port)
	log.Fatal(http.ListenAndServe("localhost"+s.Port, nil))
}

func (s *Server) handleWsConn(w http.ResponseWriter, r *http.Request) {
	tmp, err := s.Upgrade(w, r, nil)
	conn := NewConn(tmp)
	if err != nil {
		s.DebugPrintln(err)
		err = nil
	}
	s.mu.Lock()
	s.Conns[conn.RemoteAddr().String()] = conn
	s.mu.Unlock()
	s.DebugPrintf("new connection established (%s)\n", conn.RemoteAddr().String())
	s.wg.Add(1)
	go s.readLoop(conn)
	s.wg.Wait()
}

func (s *Server) Close(conn *Conn) {
	err := conn.Close()
	if err != nil {
		log.Println("connection closing error:", err)
	} else {
		closeMsg := fmt.Sprintf("closed connection (%s)", conn.RemoteAddr().String())
		if conn.CurrUser != nil {
			closeMsg += fmt.Sprintf(", (%s)", conn.CurrUser.Username)
		}
		log.Println(closeMsg)
		delete(s.Conns, conn.RemoteAddr().String())
		if conn.CurrUser != nil {
			delete(s.Users, conn.CurrUser.Username)
			conn.CurrUser = nil
		}
	}
}

func (s *Server) readLoop(conn *Conn) {
	defer s.wg.Done()
	defer s.Close(conn)
	for {
		var data Data
		err := conn.ReadJSON(&data)
		if err != nil {
			s.DebugPrintln("read error:", err)
			if _, ok := err.(*net.OpError); ok || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}
		}
		msg := NewMessage(conn.RemoteAddr().String(), data)
		s.handleMsg(msg)
	}
}

func (s *Server) handleMsg(msg Message) {
	marshal, _ := json.Marshal(msg.Data)
	s.DebugPrintf("new message received from (%s): %s\n", msg.From, string(marshal))
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Data.RequestType {
	case CreateUser:
		s.handleCreateUser(msg.Data, msg.From)
	case DeleteUser:
		s.handleDeleteUser(msg.Data)
	case UpdateUser:
		s.handleUpdateUser(msg.Data)
	case CallUpdate:
		s.handleCallUpdate(msg.Data.CallData, msg.From)
	case GetUsers:
		s.handleGetUsers(msg.From)
	default:
		s.Conns[msg.From].sendErr(
			BadRequestError,
			fmt.Sprintf("received invalid request_type: (%s)", msg.Data.RequestType),
		)
		s.DebugPrintf("received invalid request_type: (%s)", msg.Data.RequestType)
	}
}
