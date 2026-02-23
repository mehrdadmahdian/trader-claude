package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

// MessageType identifies the WebSocket message category
type MessageType string

const (
	MsgTypeTick         MessageType = "tick"
	MsgTypeCandle       MessageType = "candle"
	MsgTypeSignal       MessageType = "signal"
	MsgTypeAlert        MessageType = "alert"
	MsgTypeNotification MessageType = "notification"
	MsgTypeError        MessageType = "error"
	MsgTypePing         MessageType = "ping"
	MsgTypePong         MessageType = "pong"
)

// Message is the envelope for all WebSocket messages
type Message struct {
	Type    MessageType     `json:"type"`
	Channel string          `json:"channel,omitempty"` // e.g. "ticks:BTC/USDT"
	Payload json.RawMessage `json:"payload"`
}

// Client represents a connected WebSocket client
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	hub       *Hub
	subscribe map[string]bool // subscribed channels
	mu        sync.Mutex
}

// Hub manages all active WebSocket connections
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
}

var hubOnce sync.Once
var hubInstance *Hub

// NewHub creates or returns the singleton Hub
func NewHub() *Hub {
	hubOnce.Do(func() {
		hubInstance = &Hub{
			clients:    make(map[*Client]bool),
			broadcast:  make(chan *Message, 256),
			register:   make(chan *Client),
			unregister: make(chan *Client),
		}
	})
	return hubInstance
}

// Run starts the hub event loop — call this in a goroutine
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[ws] client connected, total=%d", h.clientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[ws] client disconnected, total=%d", h.clientCount())

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("[ws] marshal error: %v", err)
				continue
			}
			h.mu.RLock()
			for client := range h.clients {
				client.mu.Lock()
				subscribed := client.subscribe[msg.Channel] || msg.Channel == ""
				client.mu.Unlock()
				if !subscribed {
					continue
				}
				select {
				case client.send <- data:
				default:
					// slow client — drop message
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all subscribed clients
func (h *Hub) Broadcast(msg *Message) {
	h.broadcast <- msg
}

// BroadcastJSON is a helper that marshals payload and broadcasts
func (h *Hub) BroadcastJSON(msgType MessageType, channel string, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	h.Broadcast(&Message{
		Type:    msgType,
		Channel: channel,
		Payload: raw,
	})
	return nil
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS handles a new WebSocket connection
func (h *Hub) ServeWS(c *websocket.Conn) {
	client := &Client{
		conn:      c,
		send:      make(chan []byte, 256),
		hub:       h,
		subscribe: make(map[string]bool),
	}
	h.register <- client

	// Writer goroutine
	go func() {
		for data := range client.send {
			if err := c.WriteMessage(1, data); err != nil {
				break
			}
		}
		c.Close()
	}()

	// Reader loop
	defer func() {
		h.unregister <- client
	}()

	for {
		mt, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		if mt != 1 { // only text frames
			continue
		}

		var msg struct {
			Action  string `json:"action"`  // "subscribe" | "unsubscribe" | "ping"
			Channel string `json:"channel"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		client.mu.Lock()
		switch msg.Action {
		case "subscribe":
			client.subscribe[msg.Channel] = true
		case "unsubscribe":
			delete(client.subscribe, msg.Channel)
		case "ping":
			pong, _ := json.Marshal(Message{Type: MsgTypePong})
			client.send <- pong
		}
		client.mu.Unlock()
	}
}
