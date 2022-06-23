package game

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
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
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Player struct {
	Name     string
	Score    int
	IsLeader bool
	Room     *Room
	Conn     *websocket.Conn
	send     chan []byte
}

type JSONPlayer struct {
	Name     string `json:"name"`
	Score    int    `json:"score"`
	IsLeader bool   `json:"isLeader"`
}

type PlayerAction struct {
	Action string `json:"action"`
	Value  int    `json:"value"`
}

func (p *Player) ToJSONPlayer() *JSONPlayer {
	return &JSONPlayer{Name: p.Name, Score: p.Score, IsLeader: p.IsLeader}
}

func (p *Player) Vote(vote int) {
	question := p.Room.Questions[p.Room.CurrentQuestion]
	if _, ok := question.Answers[p]; !ok && p.Room.Scene == 1 {
		if p.Room.CurrentQuestion >= len(p.Room.Questions) {
			return
		}

		question.Answers[p] = vote
		// TODO: Needed for seeing who voted?
		p.Room.BroadcastRoomState()
	}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (p *Player) readPump() {
	defer func() {
		p.Room.unregister <- p
		p.Conn.Close()
	}()
	p.Conn.SetReadLimit(maxMessageSize)
	p.Conn.SetReadDeadline(time.Now().Add(pongWait))
	p.Conn.SetPongHandler(func(string) error { p.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		action := PlayerAction{}
		err := p.Conn.ReadJSON(&action)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Msg("Room [" + p.Room.ID + "]: Player " + p.Name + " websocket closed unexpectedly")
			}
			log.Warn().Err(err).Msg("Room [" + p.Room.ID + "]: Could not read JSON message from player " + p.Name)
			break
		}
		log.Debug().Msg("Room [" + p.Room.ID + "]: " + p.Name + " performed action {" + action.Action + "} with value " + fmt.Sprint(action.Value))
		if action.Action == "Vote" {
			p.Vote(action.Value)
		} else if action.Action == "Start" {
			// Leader can start the game if the game is not yet started
			// or if the game is over.
			if p.IsLeader && (p.Room.Scene == 0 || p.Room.Scene == 3) {
				go p.Room.StartGame()
			}
		}
		//p.Room.broadcast <- p.Room.ToJSON()
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (p *Player) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		p.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-p.send:
			p.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				p.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := p.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			// n := len(p.send)
			// for i := 0; i < n; i++ {
			// 	w.Write(newline)
			// 	w.Write(<-p.send)
			// }

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			p.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func ServeWs(room *Room, isLeader bool, playerName string, w http.ResponseWriter, r *http.Request) error {
	// if name is already taken
	for p := range room.Players {
		if p.Name == playerName {
			return errors.New("name already taken")
		}
	}

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Could not upgrade websocket connection")
		return err
	}
	if playerName == "" {
		playerName = "Player " + fmt.Sprint(len(room.Players)+1)
	}

	player := &Player{Room: room, Score: 0, IsLeader: isLeader, Conn: conn, send: make(chan []byte, 256), Name: playerName}
	player.Room.register <- player

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go player.writePump()
	go player.readPump()
	return nil
}
