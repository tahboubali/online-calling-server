package server

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"online-calling/util"
	"strings"
)

const AuthHeaderKey = "Auth-Key"

type loginSignupInfo struct {
	Username string
	Password string
}

func (lsi loginSignupInfo) validate() error {
	if err := util.IsValidPassword(lsi.Password); err != nil {
		return err
	}
	return util.IsValidUsername(lsi.Username)
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var signupInfo loginSignupInfo
	json.NewDecoder(r.Body).Decode(&signupInfo)
	if err := signupInfo.validate(); err != nil {
		errMsg := strings.ToUpper(err.Error()[:1]) + err.Error()[1:] + "."
		fmt.Fprintln(w, errMsg)
		return
	}
	if _, exists := s.Users[signupInfo.Username]; exists {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "User with username '%s' already exists.\n", signupInfo.Username)
		return
	}
	pwd := hashPassword(signupInfo.Password, s.Debugger)
	if pwd == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Failed to save password.")
		return
	}
	user := NewUser(UserInfo{Username: signupInfo.Username})
	user.HashedPassword = pwd
	user.AuthToken = NewAuthToken()
	w.WriteHeader(http.StatusOK)
	response, _ := json.Marshal(map[string]any{
		"message": "Successfully signed up user",
		"auth":    user.AuthToken,
	})
	fmt.Fprintln(w, string(response))
	userJson, _ := json.Marshal(user)
	log.Println("signed up new user:", userJson)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var loginInfo loginSignupInfo
	json.NewDecoder(r.Body).Decode(&loginInfo)
	if err := loginInfo.validate(); err != nil {
		errMsg := strings.ToUpper(err.Error()[:1]) + err.Error()[1:] + "."
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, errMsg)
		return
	}
	s.mu.Lock()
	user, exists := s.Users[loginInfo.Username]
	s.mu.Unlock()
	if !exists {
		fmt.Fprintf(w, "Invalid username.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	auth, err := user.ValidatePasswordHash(loginInfo.Password)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Invalid password.")
		return
	}
	w.WriteHeader(http.StatusOK)
	response, _ := json.Marshal(map[string]any{
		"message": "Successfully login",
		"auth":    auth,
	})
	fmt.Fprintln(w, response)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)
	auth := user.AuthToken.Auth
	s.mu.Lock()
	delete(s.usersByAuth, auth)
	delete(s.Users, user.Username)
	delete(s.Conns, user.CurrConn.RemoteAddr().String())
	s.mu.Unlock()
	user.CurrConn = nil
	w.WriteHeader(http.StatusOK)
	s.broadcastDeleteUser(user.Username)
	log.Printf("delete user '%s', user info: %")
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserInfo UserInfo `json:"user_info"`
	}
	user := r.Context().Value("user").(*User)
	oldInfo := user.UserInfo
	user.UserInfo = body.UserInfo
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Successfully updated user information.")
	log.Printf("user '%s' was successfully updated from %v to %v", user.Username, oldInfo, body.UserInfo)
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoomId int `json:"room_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	room := s.Rooms[body.RoomId]
	writeUsers := make(map[string]*User)
	s.mu.Lock()
	maps.Copy(writeUsers, room.Users)
	s.mu.Lock()
	delete(writeUsers, s.Conns[r.RemoteAddr].CurrUser.Username)
	s.DebugPrintf("sent current other users (%v) to addr (%s)", writeUsers, r.RemoteAddr)
	marshal, _ := json.Marshal(writeUsers)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, string(marshal))
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		RoomName string `json:"room_name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	s.mu.Lock()
	user := s.Users[body.Username]
	s.Rooms[s.currRoomId] = NewRoom(s.currRoomId, s.Users[body.Username], body.RoomName)
	user.RoomId = s.currRoomId
	s.mu.Unlock()
	message, _ := json.Marshal(map[string]any{
		"message": fmt.Sprintf("Successfully created room '%s' with '%s' as owner.", body.RoomName, body.Username),
		"owner":   body.Username,
		"room_id": s.currRoomId,
	})
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, string(message))
	s.mu.Lock()
	s.currRoomId++
	s.mu.Unlock()
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoomId int `json:"room_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	user := r.Context().Value("user").(*User)
	s.mu.Lock()
	s.Rooms[body.RoomId].Users[user.Username] = user
	room, exists := s.Rooms[body.RoomId]
	s.mu.Unlock()
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Room does not exist")
		return
	}
	user.RoomId = body.RoomId
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Successfully joined room '%s'\n", room.Name)
	if user.CurrConn == nil || !user.CurrConn.ready() {
		conn, _ := s.Upgrade(w, r, nil)
		user.CurrConn = NewConn(conn)
		s.mu.Lock()
		s.Conns[conn.RemoteAddr().String()] = user.CurrConn
		s.mu.Unlock()
	}
	s.joinRoom(user, body.RoomId)
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)
	var body struct {
		RoomId int
	}
	s.mu.Lock()
	room, exists := s.Rooms[body.RoomId]
	s.mu.Unlock()
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Room does not exist.")
		return
	}
	if user.Username != room.Owner.Username {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, "Permission to delete room denied.")
		return
	}
	w.WriteHeader(http.StatusOK)
	s.mu.Lock()
	//TODO add broadcast delete room for this route
	for _, user := range s.Rooms[body.RoomId].Users {
		user.RoomId = -1
	}
	delete(s.Rooms, body.RoomId)
	s.mu.Unlock()
	fmt.Fprintf(w, "Successfully deleted room '%s'", room.Name)
}

func (s *Server) handleGetRooms(w http.ResponseWriter, _ *http.Request) {
	response, _ := json.Marshal(s.Users)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, response)
}
