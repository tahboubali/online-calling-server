package server

import (
	"errors"
	"fmt"
	"slices"
)

func (s *Server) broadcastCreateUser(userInfo UserInfo) error {
	return s.broadcastJSON(map[string]any{
		"response_type": CreateUser,
		"user_info":     userInfo,
	}, userInfo.Username)
}

func (s *Server) broadcastUpdateUser(username string, userInfo UserInfo) error {
	return s.broadcastJSON(map[string]any{
		"response_type": UpdateUser,
		"username":      username,
		"user_info":     userInfo,
	}, username)
}

func (s *Server) broadcastDeleteUser(username string) error {
	return s.broadcastJSON(map[string]any{
		"response_type": DeleteUser,
		"username":      username,
	}, username)
}

func (s *Server) broadcastCallUpdate(username string, data CallData) {
	err := s.broadcastJSON(map[string]any{
		"response_type": CallUpdate,
		"username":      username,
		"call_data":     data,
	}, username)
	if err != nil {
		s.DebugPrintln("failed to broadcast call update")
	}
}

// broadcastJSON this function takes in an optional varargs list of users that the broadcast shouldn't receive
func (s *Server) broadcastJSON(data any, usernames ...string) error {
	for _, conn := range s.Conns {
		if conn.CurrUser != nil && slices.Contains(usernames, conn.CurrUser.Username) {
			continue
		}
		err := conn.WriteJSON(data)
		if err != nil {
			msg := fmt.Sprintf("could not write JSON data to addr (%s): %s", conn.RemoteAddr().String(), err)
			if conn.CurrUser != nil {
				msg += fmt.Sprintf(", user: '%s'", conn.CurrUser.Username)
			}
			writeErr := errors.New(msg)
			s.DebugPrintln(writeErr)
			return writeErr
		}
	}
	return nil
}
