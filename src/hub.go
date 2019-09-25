// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	rooms map[string]*Room

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan RoomMetaData

	// Unregister requests from clients.
	unregister chan *Room
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan RoomMetaData),
		unregister: make(chan *Room),
		rooms:      make(map[string]*Room),
	}
}

func (h *Hub) run() {
	for {
		select {
		case roomData := <-h.register:
			h.rooms[roomData.id] = roomData.room
			//case client := <-h.unregister:
			//	if _, ok := h.clients[client]; ok {
			//		delete(h.clients, client)
			//		close(client.send)
			//	}
			//case message := <-h.broadcast:
			//	for client := range h.clients {
			//		select {
			//		case client.send <- message:
			//		default:
			//			close(client.send)
			//			delete(h.clients, client)
			//		}
			//	}
		}
	}
}
