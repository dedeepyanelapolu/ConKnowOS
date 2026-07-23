package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"contextos/kernel/internal/a2a"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development dashboard connections
	},
}

// Client represents a connected WebSocket client.
type Client struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// WSHub maintains active WebSocket clients and forwards pub/sub messages from the A2A bus.
type WSHub struct {
	mu         sync.Mutex
	clients    map[*Client]bool
	eventBus   *a2a.EventBus
	subState   a2a.SubscriptionChan
	subTokens  a2a.SubscriptionChan
	subMemory  a2a.SubscriptionChan
}

// NewWSHub creates a new WSHub and registers subscriptions to the EventBus.
func NewWSHub(eventBus *a2a.EventBus) *WSHub {
	h := &WSHub{
		clients:  make(map[*Client]bool),
		eventBus: eventBus,
	}

	if eventBus != nil {
		h.subState = eventBus.Subscribe("EVENT_STATE_CHANGED")
		h.subTokens = eventBus.Subscribe("EVENT_TOKEN_COMPACTED")
		h.subMemory = eventBus.Subscribe("EVENT_MEMORY_RETRIEVED")
		go h.listenToEventBus()
	}

	return h
}

// listenToEventBus forwards messages from EventBus channels to the WSHub broadcast pump.
func (h *WSHub) listenToEventBus() {
	for {
		select {
		case event, ok := <-h.subState:
			if ok {
				h.Broadcast(event)
			}
		case event, ok := <-h.subTokens:
			if ok {
				h.Broadcast(event)
			}
		case event, ok := <-h.subMemory:
			if ok {
				h.Broadcast(event)
			}
		}
	}
}

// Broadcast marshals and dispatches data to all connected clients.
func (h *WSHub) Broadcast(event interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var data []byte
	var err error

	if bytes, ok := event.([]byte); ok {
		data = bytes
	} else {
		data, err = json.Marshal(event)
		if err != nil {
			log.Printf("[WSHub] json marshal error: %v", err)
			return
		}
	}

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// ServeHTTP upgrades the HTTP request to WebSocket protocol.
func (h *WSHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WSHub] WebSocket Upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.mu.Lock()
		delete(c.hub.clients, c)
		c.hub.mu.Unlock()
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			break
		}
	}
}
