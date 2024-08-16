package server

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"log"
	"net"
	"net/http"
	"online-calling/debug"
	"os"
	"sync"
	"time"
)

type Server struct {
	*debug.Debugger
	websocket.Upgrader
	Port  string
	wg    sync.WaitGroup
	mu    sync.Mutex
	Rooms map[int]*Room
	Conns map[string]*Conn
	// Users is indexed by username
	Users       map[string]*User
	usersByAuth map[string]*User
	currRoomId  int
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
		Rooms:       make(map[int]*Room),
		Users:       make(map[string]*User),
		Conns:       make(map[string]*Conn),
		usersByAuth: make(map[string]*User),
	}
}

type Room struct {
	Id    int
	Name  string
	Users map[string]*User
	Owner *User
	mu    sync.Mutex
}

func NewRoom(id int, owner *User, RoomName string) *Room {
	users := make(map[string]*User)
	users[owner.Username] = owner
	return &Room{
		Name:  RoomName,
		Id:    id,
		Owner: owner,
		Users: users,
	}
}

type Data struct {
	UserInfo UserInfo `json:"user_info"`
	CallData CallData `json:"call_data"`
	Username string   `json:"username"`
}

type CallData struct {
	CurrFrame string `json:"curr_frame"`
	ImgFmt    string `json:"image_format"`
}

type CallUpdate struct {
	UserInfo UserInfo
	CallData CallData
}

func NewCallUpdate(userInfo UserInfo, callData CallData) CallUpdate {
	return CallUpdate{
		UserInfo: userInfo,
		CallData: callData,
	}
}

type UserInfo struct {
	Username string `json:"username"`
}

type User struct {
	UserInfo `json:"user_info"`
	*HashedPassword
	AuthToken *AuthToken `json:"auth"`
	RoomId    int        `json:"room_id"`
	CurrConn  *Conn      `json:"-"`
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

func (s *Server) init() {
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
	s.Port = ":" + os.Getenv("PORT")
}

func (s *Server) Run() {
	s.init()
	r := mux.NewRouter()
	r.HandleFunc("/join-room", s.handleJoinRoom).Methods(http.MethodPost)
	r.HandleFunc("/create-room", s.handleCreateRoom).Methods(http.MethodPost)
	r.HandleFunc("/delete-room", s.handleDeleteRoom).Methods(http.MethodDelete)
	r.HandleFunc("/get-rooms", s.handleGetRooms).Methods(http.MethodGet)
	r.HandleFunc("/"+RequestDeleteUser, s.handleDeleteUser).Methods(http.MethodDelete)
	r.HandleFunc("/"+RequestSignup, s.handleSignup).Methods(http.MethodPost)
	r.HandleFunc("/"+RequestUpdateUser, s.handleUpdateUser).Methods(http.MethodPut, http.MethodPost)
	r.HandleFunc("/get-users", s.handleGetUsers).Methods(http.MethodGet)
	r.Use(s.LoggingMiddleware)
	r.Use(s.AuthMiddleware)
	log.Println("server started on", s.Port)
	log.Fatal(http.ListenAndServe(s.Port, r))
}

func (s *Server) closeConn(conn *Conn) {
	err := conn.Close()
	if err != nil {
		log.Println("connection closing error:", err)
	} else {
		closeMsg := fmt.Sprintf("closed connection (%s)", conn.RemoteAddr().String())
		if conn.CurrUser != nil {
			closeMsg += fmt.Sprintf(", (%s)", conn.CurrUser.Username)
		}
		log.Println(closeMsg)
		s.mu.Lock()
		delete(s.Conns, conn.RemoteAddr().String())
		if conn.CurrUser != nil {
			delete(s.Users, conn.CurrUser.Username)
			conn.CurrUser = nil
		}
		s.mu.Unlock()
	}
}

func (s *Server) joinRoom(user *User, roomId int) {
	conn := user.CurrConn
	defer s.wg.Done()
	defer s.closeConn(conn)
	for {
		var callData CallData
		err := conn.ReadJSON(&callData)
		if err != nil {
			s.DebugPrintln("read error:", err)
			var opError *net.OpError
			if errors.As(err, &opError) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}
		}
		s.broadcastCallUpdate(roomId, NewCallUpdate(user.UserInfo, callData))
	}
}

func (c *Conn) ready() bool {
	return c.readyToWrite() && c.readyToRead()
}

func (c *Conn) readyToRead() bool {
	c.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	_, _, err := c.NextReader()
	c.SetReadDeadline(time.Time{})
	return err == nil
}

func (c *Conn) readyToWrite() bool {
	c.SetWriteDeadline(time.Now().Add(1 * time.Millisecond))
	_, _, err := c.NextReader()
	c.SetReadDeadline(time.Time{})
	return err == nil
}
