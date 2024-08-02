package server

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strconv"
)

func (s *Server) handleCreateUser(data Data, from string) {
	userInfo := data.UserInfo
	username := userInfo.Username
	conn := s.Conns[from]
	if _, exists := s.Users[username]; exists {
		conn.sendErr(BadRequestError, fmt.Sprintf("User with username '%s' already exists", username))
		return
	}
	user := NewUser(userInfo)
	conn.CurrUser = user
	s.Users[username] = user
	user.CurrConn = conn
	if err := s.broadcastCreateUser(user.UserInfo); err != nil {
		conn.sendErr(InternalError, "Failed to broadcast create user message to other users.")
		return
	}
	s.DebugPrintf("created new user: '%s'\n", username)
	conn.sendSuccess(fmt.Sprintf("User '%s' was successfully created.", username))
}

func (s *Server) handleDeleteUser(data Data) {
	username := data.Username
	user := s.Users[username]
	conn := user.CurrConn
	if err := s.broadcastDeleteUser(username); err != nil {
		conn.sendErr(InternalError, "Failed to broadcast delete user message to other users.")
		return
	}
	delete(s.Users, username)
	conn.CurrUser = nil
	s.DebugPrintf("deleted user: '%s'\n", username)
	conn.sendSuccess(fmt.Sprintf("User '%s' was successfully deleted.", username))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Invalid request given"))
		return
	}
	var data Data
	json.NewDecoder(r.Body).Decode(&data)
	username := data.Username
	userInfo := data.UserInfo
	user := s.Users[username]
	conn := user.CurrConn
	user.UserInfo = userInfo
	if err := s.broadcastUpdateUser(username, userInfo); err != nil {
		conn.sendErr(InternalError, "Failed to broadcast.")
	}
	s.DebugPrintf("updated user: from '%s' to '%s'", username, userInfo.Username)
	conn.sendSuccess(fmt.Sprintf("User was successfully updated from '%s' to '%s'.",
		username,
		userInfo.Username),
	)
}

func (s *Server) handleCallUpdate(callData CallData, from string) {
	if s.Conns[from].CurrUser == nil {
		return
	}
	username := s.Conns[from].CurrUser.Username
	s.broadcastCallUpdate(username, callData)
	s.DebugPrintf("new update received from: '%s'", username)
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Invalid request type received."))
		return
	}
	if s.Conns[r.RemoteAddr].CurrUser == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request; user has not been registered."))
		return
	}
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	room := s.Rooms[id]
	writeUsers := make(map[string]*User)
	maps.Copy(writeUsers, room.Users)
	delete(writeUsers, s.Conns[r.RemoteAddr].CurrUser.Username)
	s.DebugPrintf("sent current other users (%v) to addr (%s)", writeUsers, r.RemoteAddr)
	marshal, _ := json.Marshal(writeUsers)
	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintln(w, string(marshal))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}
