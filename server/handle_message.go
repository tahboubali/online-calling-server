package server

import "fmt"

func (s *Server) handleCreateUser(data Data, from string) {
	userInfo := data.UserInfo
	username := userInfo.Username
	user := NewUser(userInfo)
	conn := s.Conns[from]
	conn.CurrUser = user
	s.Users[username] = user
	user.CurrConn = conn
	if err := s.broadcastCreateUser(user.UserInfo); err != nil {
		conn.sendErr(Error, "Failed to broadcast create user message to other users.")
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
		conn.sendErr(Error, "Failed to broadcast delete user message to other users.")
		return
	}
	delete(s.Users, username)
	conn.CurrUser = nil
	s.DebugPrintf("deleted user: '%s'\n", username)
	conn.sendSuccess(fmt.Sprintf("User '%s' was successfully deleted.", username))
}

func (s *Server) handleUpdateUser(data Data) {
	username := data.Username
	userInfo := data.UserInfo
	user := s.Users[username]
	conn := user.CurrConn
	user.UserInfo = userInfo
	if err := s.broadcastUpdateUser(username, userInfo); err != nil {
		conn.sendErr(Error, "Failed to broadcast ")
	}
	s.DebugPrintf("updated user: from '%s' to '%s'", username, userInfo.Username)
	conn.sendSuccess(fmt.Sprintf("User was successfully updated from '%s' to '%s'.",
		username,
		userInfo.Username),
	)
}

func (s *Server) handleCallUpdate(data Data) {
	username := data.Username
	s.broadcastCallUpdate(username, data.CallData)
	s.DebugPrintf("new update received from: '%s'", username)
}

func (s *Server) handleGetUsers(from string) {
	conn := s.Conns[from]
	username := s.Conns[from].CurrUser.Username
	err := conn.WriteJSON(s.Users)
	if err != nil {
		s.DebugPrintf("error writing get users to user '%s': %s\n", username, err)
	}
	printUsername := "UNREGISTERED CONNECTION"
	if username != "" {
		printUsername = username
	}
	s.DebugPrintf("sent current users to addr (%s), username '%s'", conn.RemoteAddr().String(), printUsername)
}
