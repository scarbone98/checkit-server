// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type ClientState string

const (
	MATCHED ClientState = "MATCHED"
	ACCEPTED ClientState = "ACCEPTED"
	SEARCHING ClientState = "SEARCHING"
	IDLE      ClientState = "IDLE"
)

type ActionType string

const (
	Search ActionType = "SEARCH"
	AcceptedMatch ActionType = "ACCEPT_MATCH"
	DeclineMatch ActionType = "DECLINE_MATCH"
	SendMessage ActionType = "SEND_MESSAGE"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	room *Room

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	state ClientState

	id string

	gender string

	partner *Client

}

type Action struct {
	Type    ActionType
	Payload map[string]interface{}
	Sender *Client
}

type RequestWrapper struct {
	Action   Action
	SenderId string
}

func closeConnections(c *Client)  {
	if c.partner != nil {
		data := serializeData(map[string]interface{}{"Type": "PARTNER_DISCONNECTED", "Payload": nil,})
		c.partner.send <- data
		c.partner.partner = nil
		c.partner = nil
	}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.room.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {

	}
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			fmt.Println(err)
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var action Action
		if err := json.Unmarshal(message, &action); err != nil {
			fmt.Println(err.Error())
			continue
		}
		action.Sender = c
		fmt.Println("ACTION", &action)
		c.room.broadcast <- &action
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.partner = nil
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			fmt.Println("Checking connection")
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				fmt.Println("CLIENT DISCONNECTED")
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func connectClient(conn *websocket.Conn, room *Room) {
	client := &Client{room: room, conn: conn, send: make(chan []byte, 256), state: IDLE}
	client.room.register <- client
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.

	// Broadcast that a new client is searching for a match
	client.room.broadcast <- &Action{
		Type:    Search,
		Payload: map[string]interface{}{"data": "test", "client": client},
		Sender: client,
	}
	client.state = SEARCHING
	go client.writePump()
	go client.readPump()
}
