package game

import (
	"encoding/json"
	"math/rand"
	s "strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Room struct {
	ID              string
	Players         map[*Player]bool
	Questions       []*Question
	NQuestions      int
	CurrentQuestion int
	// 0 = not started, 1 = question time, 2 = question results, 3 = game over
	Scene  int
	Active bool

	// Inbound messages from the clients.
	broadcast chan []byte
	// Register requests from the clients.
	register chan *Player
	// Unregister requests from clients.
	unregister chan *Player
	// Kill room if true
	kill chan bool
}

type JSONRoom struct {
	ID              string          `json:"id"`
	Players         []*JSONPlayer   `json:"players"`
	Questions       []*JSONQuestion `json:"questions"`
	CurrentQuestion int             `json:"current_question"`
	Scene           int             `json:"scene"`
}

func NewRoom(roomID string) *Room {
	return &Room{
		broadcast:       make(chan []byte),
		register:        make(chan *Player),
		unregister:      make(chan *Player),
		Players:         make(map[*Player]bool),
		ID:              roomID,
		Questions:       []*Question{},
		NQuestions:      15,
		CurrentQuestion: 0,
		Scene:           0,
		Active:          true,
	}
}

func (r *Room) ToJSON() []byte {
	jsonRoom := &JSONRoom{ID: r.ID, Players: []*JSONPlayer{}, Questions: []*JSONQuestion{}, CurrentQuestion: r.CurrentQuestion, Scene: r.Scene}

	for player := range r.Players {
		jsonRoom.Players = append(jsonRoom.Players, player.ToJSONPlayer())
	}
	for _, question := range r.Questions {
		jsonRoom.Questions = append(jsonRoom.Questions, question.ToJSONQuestion())
	}

	b, err := json.Marshal(jsonRoom)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal room to JSON")
	}
	return b
}

func (r *Room) AddPlayer(player *Player) {
	r.Players[player] = true
	log.Debug().Msg("Room [" + r.ID + "]: Added " + player.Name)
	r.BroadcastRoomState()
}

func (r *Room) RemovePlayer(player *Player) {
	if _, ok := r.Players[player]; ok {
		delete(r.Players, player)
		close(player.send)
		log.Debug().Msg("Room [" + r.ID + "]: Removed " + player.Name)
		r.BroadcastRoomState()
	}
}

func (r *Room) selectNQuestions(num int) {
	if len(r.Questions) < num {
		num = len(r.Questions)
	}
	r.Questions = r.Questions[:num]
}

func (r *Room) shuffleQuestions() {
	rand.Seed(time.Now().UnixNano())
	for i := len(r.Questions) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		r.Questions[i], r.Questions[j] = r.Questions[j], r.Questions[i]
	}
}

func (r *Room) parseQuestions() {
	parsedQuestions := []*Question{}

	playerNames := []string{}
	for player := range r.Players {
		playerNames = append(playerNames, player.Name)
	}

	rand.Seed(time.Now().UnixNano())
	for _, question := range r.Questions {
		player1 := playerNames[rand.Intn(len(playerNames))]
		player2 := playerNames[rand.Intn(len(playerNames))]
		for player2 == player1 {
			player2 = playerNames[rand.Intn(len(playerNames))]
		}

		// TODO: Fix so that "{1/2}s" don't become e.g. Pontuss
		desc := s.ReplaceAll(question.Description, "{1}", player1)
		desc = s.ReplaceAll(desc, "{2}", player2)
		for i, choice := range question.Choices {
			choice = s.ReplaceAll(choice, "{1}", player1)
			choice = s.ReplaceAll(choice, "{2}", player2)
			question.Choices[i] = choice
		}

		parsedQuestion := &Question{
			Type:        question.Type,
			Description: desc,
			Choices:     question.Choices,
			Answers:     question.Answers,
			Reward:      question.Reward,
		}
		parsedQuestions = append(parsedQuestions, parsedQuestion)
	}

	r.Questions = parsedQuestions
}

func (r *Room) NextQuestion() *Question {
	if r.CurrentQuestion >= len(r.Questions)-1 {
		return nil
	}
	r.CurrentQuestion++

	return r.Questions[r.CurrentQuestion]
}

func (r *Room) ResetGame() {
	r.Scene = 0
	r.CurrentQuestion = 0
	for _, question := range r.Questions {
		question.Answers = make(map[*Player]int)
	}
}

func (r *Room) BroadcastRoomState() {
	for player := range r.Players {
		select {
		case player.send <- r.ToJSON():
		default:
			close(player.send)
			delete(r.Players, player)
		}
	}
}

func (r *Room) StartGame() {
	r.parseQuestions()
	r.shuffleQuestions()
	r.selectNQuestions(r.NQuestions)
	r.Scene = 1
	r.BroadcastRoomState()

	prevScene := 0
	for {
		if len(r.Players) == 0 {
			log.Debug().Msg("Room [" + r.ID + "]: No players left, killing room...")
			r.Active = false
			r.kill <- true
			return
		}

		// Move to results screen if every player has answered
		if len(r.Questions[r.CurrentQuestion].Answers) == len(r.Players) && r.Scene == 1 && len(r.Players) > 0 { // TODO: Remove len(r.Players) > 0
			r.Scene = 2
			//r.BroadcastRoomState()
		}
		// If scene has changed, broadcast the new scene
		if r.Scene != prevScene {
			switch r.Scene {
			case 0:
				prevScene = 0
			// Question time
			case 1:
				prevScene = 1
				log.Debug().Msg("Room [" + r.ID + "]: Starting question (" + r.Questions[r.CurrentQuestion].Description + ")")
			// Question results
			case 2:
				prevScene = 2
				log.Debug().Msg("Room [" + r.ID + "]: Displaying results for (" + r.Questions[r.CurrentQuestion].Description + ")")
				r.Questions[r.CurrentQuestion].AwardScores()
				r.BroadcastRoomState()
				time.Sleep(time.Second * 15)
				if r.NextQuestion() == nil {
					r.Scene = 3
				} else {
					r.Scene = 1
				}
			// Game over
			case 3:
				prevScene = 3
				log.Debug().Msg("Room [" + r.ID + "]: Game over")
				r.Active = false
				//time.Sleep(time.Second * 5)
			}
			// TODO: Should it work like this?
			if r.Scene == 3 {
				r.Active = false
				r.BroadcastRoomState()
				log.Debug().Msg("Room [" + r.ID + "]: Shutting down...")
				r.kill <- true
				break
			}
			r.BroadcastRoomState()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (r *Room) Run() {
	for {
		select {
		case player := <-r.register:
			r.AddPlayer(player)
		case player := <-r.unregister:
			if r.Scene != 3 {
				r.RemovePlayer(player)
			}
		case message := <-r.broadcast:
			for player := range r.Players {
				select {
				case player.send <- message:
				default:
					close(player.send)
					delete(r.Players, player)
				}
			}
		// TODO: This never triggers?
		case kill := <-r.kill:
			if kill {
				log.Debug().Msg("Room [" + r.ID + "]: Trying to kill...")
				return
			}
		}
	}
}
