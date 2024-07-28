package server_test

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"os"
	"testing"
)

func TestCreateUser(t *testing.T) {
	dialer := &websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, _, _ := dialer.Dial("ws://localhost:8080/ws", nil)
	//defer conn.Close()
	file, err := os.Open("/Users/alitahboub/GolandProjects/online-calling/json_testing_examples/new-user.json")
	defer file.Close()
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err := json.NewDecoder(file).Decode(&request); err != nil {
		t.Fatal("error decoding json file:", err)
	}
	t.Logf("sending %s to the server", request)
	conn.WriteJSON(request)
}
