package server

import (
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
		"data":          data,
	}, username)
	if err != nil {
		s.DebugPrintln("failed to broadcast call update")
	}
}

// broadcastJSON this function takes in an optional varargs list of users that the broadcast shouldn't receive
func (s *Server) broadcastJSON(data any, usernames ...string) error {
	for _, conn := range s.Conns {
		if slices.Contains(usernames, conn.CurrUser.Username) {
			continue
		}
		err := conn.WriteJSON(data)
		if err != nil {
			errf := fmt.Errorf("could not write JSON data to user %s: %s", conn.CurrUser.Username, err)
			s.DebugPrintln(errf)
			return errf
		}
	}
	return nil
}
