package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"sync"
)

const (
	CreateUser = "create-user"
	DeleteUser = "delete-user"
	UpdateUser = "update-user"
	CallUpdate = "call-update"
	GetUsers   = "get-users"
)

type Server struct {
	*Debugger
	websocket.Upgrader
	Port  string
	Conns map[string]*Conn
	Users map[string]*User
	wg    sync.WaitGroup
	mu    sync.Mutex
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

type Conn struct {
	*websocket.Conn
	CurrUser *User
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

func NewConn(conn *websocket.Conn) *Conn {
	return &Conn{
		Conn: conn,
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

func (s *Server) Init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("error loading .env file: %v", err)
	}
	debug := os.Getenv("DEBUG") == "true"
	if debug {
		log.Println("DEBUG mode is enabled (logs will be printed)")
	} else {
		log.Println("DEBUG mode is disabled (logs will not be printed)")
	}
	s.Debugger = NewDebugger(debug)
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

func (s *Server) readLoop(conn *Conn) {
	defer s.wg.Done()
	for {
		var data Data
		err := conn.ReadJSON(&data)
		if err != nil {
			s.DebugPrintln("read error:", err)
		}
		msg := NewMessage(conn.RemoteAddr().String(), data)
		s.handleMsg(msg)
	}
}

func (s *Server) handleMsg(msg Message) {
	//FIXME add error handling to dis
	marshal, _ := json.Marshal(msg.Data)
	s.DebugPrintf("new message received from (%s): %s\n", msg.From, string(marshal))
	s.mu.Lock()
	defer s.mu.Unlock()
	userInfo := msg.Data.UserInfo
	username := msg.Data.Username

	switch msg.Data.RequestType {
	case CreateUser:
		{
			user := NewUser(userInfo)
			s.Conns[msg.From].CurrUser = user
			s.Users[username] = user
			user.CurrConn = s.Conns[msg.From]
			s.broadcastCreateUser(user.UserInfo)
			s.DebugPrintf("created new user: '%s'\n", username)
			break
		}
	case DeleteUser:
		{
			user := s.Users[username]
			addr := user.CurrConn.RemoteAddr().String()
			s.Conns[addr].CurrUser = nil
			delete(s.Users, username)
			s.broadcastDeleteUser(username)
			s.DebugPrintf("deleted user: '%s'\n", username)
			break
		}
	case UpdateUser:
		{
			user := s.Users[username]
			user.UserInfo = userInfo
			s.broadcastUpdateUser(username, userInfo)
			s.DebugPrintf("updated user: from '%s' to '%s'", username, userInfo.Username)
			break
		}
	case CallUpdate:
		{
			s.broadcastCallUpdate(username, msg.Data.CallData)
			s.DebugPrintf("new update received from: '%s'", username)
			break
		}
	case GetUsers:
		{
			conn := s.Conns[msg.From]
			err := conn.WriteJSON(s.Users)
			if err != nil {
				s.DebugPrintf("error writing get users to user '%s': %s\n", username, err)
			}
			if username == "" {
				username = "unregistered"
			}
			s.DebugPrintf("sent current users to addr (%s), username '%s'", conn.RemoteAddr().String(), username)
			break
		}
	default:
		{
			s.DebugPrintf("received invalid request_type: (%s)", msg.Data.RequestType)
		}
	}
}

func (s *Server) broadcastCreateUser(userInfo UserInfo) {
	s.broadcastJSON(map[string]any{
		"request_type": CreateUser,
		"user":         userInfo,
	})
}

func (s *Server) broadcastUpdateUser(username string, userInfo UserInfo) {
	s.broadcastJSON(map[string]any{
		"request_type": UpdateUser,
		"username":     username,
		"userInfo":     userInfo,
	})
}

func (s *Server) broadcastDeleteUser(username string) {
	s.broadcastJSON(map[string]any{
		"request_type": DeleteUser,
		"username":     username,
	})
}

func (s *Server) broadcastCallUpdate(username string, data CallData) {
	s.broadcastJSON(map[string]any{
		"request_type": CallUpdate,
		"username":     username,
		"data":         data,
	})
}

func (s *Server) broadcastJSON(data any) {
	for _, conn := range s.Conns {
		err := conn.WriteJSON(data)
		if err != nil {
			s.DebugPrintf("error writing data: %s\n", err)
		}
	}
}
