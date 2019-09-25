package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

type Room struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *Action

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	hub *Hub

	conn *websocket.Conn
}

func newRoom(hub *Hub) *Room {
	return &Room{
		broadcast:  make(chan *Action),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		hub:        hub,
	}
}

type RoomMetaData struct {
	room *Room
	id   string
}

func sendDataToClient(r *Room, c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		close(c.send)
		closeConnections(c)
		delete(r.clients, c)
	}
}

func serializeData(data map[string]interface{}) []byte {
	serialized, _ := json.Marshal(data)
	return serialized
}

func relayAction(r *Room, a *Action) {
	fmt.Println(a)
	switch a.Type {
	case Search:
		for client := range r.clients {
			if client == a.Sender {
				fmt.Println("INSIDE CONTINUE")
				continue
			}
			if true {
				data := serializeData(map[string]interface{}{"Type": "MATCH_FOUND", "Payload": map[string]interface{}{"data": "test"},})
				sendDataToClient(r, client, data)
				sendDataToClient(r, a.Sender, data)
				return
			}
		}
		sendDataToClient(r, a.Sender, serializeData(map[string]interface{}{"Type": "NO_MATCH", "Payload": map[string]interface{}{"data": "test"},}))
		break
	case AcceptedMatch:
		for client := range r.clients {
			if client == a.Sender {
				fmt.Println("INSIDE CONTINUE")
				continue
			}
			if true {
				if client.state == ACCEPTED {
					data := serializeData(map[string]interface{}{"Type": "JOINED_MATCH", "Payload": map[string]interface{}{"data": "test"},})
					sendDataToClient(r, client, data)
					sendDataToClient(r, a.Sender, data)
					a.Sender.state = MATCHED
					client.state = MATCHED
					client.partner = a.Sender
					a.Sender.partner = client
					return
				}
				data := serializeData(map[string]interface{}{"Type": "PARTNER_ACCEPTED", "Payload": map[string]interface{}{"data": "test"},})
				sendDataToClient(r, client, data)
				data = serializeData(map[string]interface{}{"Type": "ACCEPTED_MATCH", "Payload": map[string]interface{}{"data": "test"},})
				sendDataToClient(r, a.Sender, data)
				a.Sender.state = ACCEPTED
				return
			}
		}
		break
	case DeclineMatch:
		for client := range r.clients {
			if client == a.Sender {
				fmt.Println("INSIDE CONTINUE")
				continue
			}
			if true {
				data := serializeData(map[string]interface{}{"Type": "PARTNER_DECLINED", "Payload": map[string]interface{}{"data": "test"},})
				sendDataToClient(r, client, data)
				data = serializeData(map[string]interface{}{"Type": "DECLINED_MATCH", "Payload": map[string]interface{}{"data": "test"},})
				sendDataToClient(r, a.Sender, data)
				a.Sender.state = SEARCHING
				client.state = SEARCHING
				return
			}
		}
		break
	case SendMessage:
		if a.Sender.partner != nil {
			partner := a.Sender.partner
			sendTime:=time.Now().Unix()
			data := serializeData(map[string]interface{}{"Type": "MESSAGE_RECEIVED", "Payload": map[string]interface{}{"message": a.Payload["message"], "from": "PARTNER", "time": sendTime},})
			partner.send <- data
			data = serializeData(map[string]interface{}{"Type": "MESSAGE_RECEIVED", "Payload": map[string]interface{}{"message": a.Payload["message"], "from": "USER", "time": sendTime},})
			a.Sender.send <- data
		} else {
			data := serializeData(map[string]interface{}{"Type": "PARTNER_DISCONNECTED", "Payload": nil,})
			a.Sender.send <- data
		}
	}

}

func (r *Room) run() {
	for {
		select {
		case client := <-r.register:
			r.clients[client] = true
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				closeConnections(client)
				close(client.send)
			}
		case action := <-r.broadcast:
			// CHECK ACTION HERE IF NEEDED
			relayAction(r, action)
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// Gets room id if there isn't a room with that id make a new room
	// else join the existing room
	id := r.Header.Get("room-id")
	fmt.Println(id)
	if room, ok := hub.rooms[id]; !ok {
		room := newRoom(hub)
		go room.run()
		room.hub.register <- RoomMetaData{room: room, id: id}
		connectClient(conn, room)
	} else {
		connectClient(conn, room)
	}
}
