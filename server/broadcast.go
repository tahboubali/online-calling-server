package server

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	RequestLogin      = "login"
	RequestSignup     = "signup"
	RequestUpdateUser = "update-user"
	RequestDeleteUser = "delete-user"
	RequestCallUpdate = "call-update"
	RequestJoinRoom   = "join-room"
)

type Broadcast struct {
	RequestType string `json:"request_type"`
	Data        any
}

func NewBroadcast(requestType string, data any) Broadcast {
	return Broadcast{
		RequestType: requestType,
		Data:        data,
	}
}

func (s *Server) broadcastJoinRoom(userInfo UserInfo, roomId int) error {
	return s.broadcastRoomUpdate(userInfo.Username, NewBroadcast(RequestJoinRoom, userInfo), roomId)
}

func (s *Server) broadcastCreateUser(userInfo UserInfo) error {
	return s.broadcastServerUpdate(NewBroadcast(RequestSignup, userInfo), notUser(userInfo.Username))
}

func (s *Server) broadcastUpdateUser(username string, userInfo UserInfo) error {
	return s.broadcastServerUpdate(NewBroadcast(RequestUpdateUser, userInfo), notUser(username))
}

func (s *Server) broadcastDeleteUser(username string) error {
}

func (s *Server) broadcastCallUpdate(roomId int, callUpdate CallUpdate) {
	err := s.broadcastRoomUpdate(callUpdate.Username, NewBroadcast(RequestCallUpdate, callUpdate.CallData), roomId)
	if err != nil {
		s.DebugPrintln("failed to broadcast call update")
	}
}

// broadcastServerUpdate this function takes in a predicate callback function that checks if a user should be sent to
func (s *Server) broadcastServerUpdate(broadcast Broadcast, shouldSend func(user *User) bool) error {
	for _, user := range s.Users {
		if !shouldSend(user) {
			continue
		}
		conn := user.CurrConn
		err := conn.WriteJSON(broadcast)
		if err != nil {
			marshal, _ := json.Marshal(broadcast)
			writeErr := errors.New(fmt.Sprintf("failed to broadcast data: %s to user '%s'", marshal, user.Username))
			s.DebugPrintln(writeErr)
			return writeErr
		}
	}
	return nil
}

func (s *Server) broadcastRoomUpdate(username string, broadcast Broadcast, roomId int) error {
	return s.broadcastServerUpdate(broadcast, func(user *User) bool {
		s.mu.Lock()
		room := s.Rooms[roomId]
		s.mu.Unlock()
		room.mu.Lock()
		_, inRoom := room.Users[user.Username]
		room.mu.Unlock()
		return inRoom && notUser(username)(user)
	})
}

func notUser(username string) func(user *User) bool {
	return func(user *User) bool {
		return user.Username == username
	}
}
