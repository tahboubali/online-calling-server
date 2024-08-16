package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (s *Server) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		conn := s.Conns[r.RemoteAddr]
		s.mu.Unlock()
		msg := fmt.Sprintf("received new request from address: (%s)", r.RemoteAddr)
		if conn.CurrUser != nil {
			msg += fmt.Sprintf(", username: \"%s\"", conn.CurrUser.Username)
		}
		msg += fmt.Sprintf(", request: %s", r.RequestURI)
		log.Println(msg)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, RequestSignup) || strings.Contains(r.URL.Path, RequestLogin) {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get(AuthHeaderKey)
		s.mu.Lock()
		user, exists := s.usersByAuth[auth]
		s.mu.Unlock()
		if !exists {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, "Invalid authentication key.")
			return
		}
		if err := user.AuthToken.IsValid(); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
