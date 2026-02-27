package api

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/trader-claude/backend/internal/monitor"
)

// wsClientMsg is the control message sent by the browser.
type wsClientMsg struct {
	Action     string  `json:"action"`      // "subscribe" | "unsubscribe"
	MonitorIDs []int64 `json:"monitor_ids"` // IDs to add/remove
}

// signalsWS handles WS /ws/monitors/signals
// Protocol:
//
//	Client → Server: {"action":"subscribe",   "monitor_ids":[1,2,3]}
//	Client → Server: {"action":"unsubscribe", "monitor_ids":[1]}
//	Server → Client: signal JSON (only for subscribed monitor IDs)
func signalsWS(rdb *redis.Client) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Subscribe to the Redis broadcast channel
		sub := rdb.Subscribe(ctx, monitor.SignalChannel)
		defer sub.Close()

		// Per-connection subscription set
		var mu sync.RWMutex
		subscribed := make(map[int64]bool)

		redisCh := sub.Channel()

		// Goroutine: read client control messages
		clientMsgs := make(chan wsClientMsg, 16)
		go func() {
			defer close(clientMsgs)
			for {
				_, raw, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var msg wsClientMsg
				if err := json.Unmarshal(raw, &msg); err != nil {
					continue
				}
				clientMsgs <- msg
			}
		}()

		for {
			select {
			case msg, ok := <-clientMsgs:
				if !ok {
					return
				}
				mu.Lock()
				switch msg.Action {
				case "subscribe":
					for _, id := range msg.MonitorIDs {
						subscribed[id] = true
					}
				case "unsubscribe":
					for _, id := range msg.MonitorIDs {
						delete(subscribed, id)
					}
				}
				mu.Unlock()

			case redisMsg, ok := <-redisCh:
				if !ok {
					return
				}
				// Parse the signal event
				var evt monitor.SignalEvent
				if err := json.Unmarshal([]byte(redisMsg.Payload), &evt); err != nil {
					log.Printf("ws/monitors/signals: malformed event: %v", err)
					continue
				}
				// Forward only if the client has subscribed to this monitor
				mu.RLock()
				shouldSend := subscribed[evt.MonitorID]
				mu.RUnlock()
				if !shouldSend {
					continue
				}
				b, err := json.Marshal(evt)
				if err != nil {
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}
}
