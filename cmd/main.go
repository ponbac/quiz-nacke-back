package main

import (
	"math/rand"
	"net/http"
	"os"
	"strconv"
	s "strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ponbac/majority-wins/data"
	"github.com/ponbac/majority-wins/game"
)

// Holds all rooms, key = room ID, value = room pointer
var rooms = map[string]*game.Room{}

func createRoom(c echo.Context) error {
	name := c.QueryParam("name")
	var roomID string
	for ok := true; ok; _, ok = rooms[roomID] {
		roomID = randomString(4)
	}

	room := game.NewRoom(roomID)
	rooms[roomID] = room
	room.Questions = fetchOpenTDBQuestions()
	nQuestions := c.QueryParam("questions")
	if nQuestions != "" {
		n, err := strconv.Atoi(nQuestions)
		if err != nil {
			log.Error().Err(err).Msg("Could not convert questions param to int")
		} else {
			room.NQuestions = n
		}
	}
	go room.Run()
	log.Debug().Msg("Room [" + roomID + "]: Created")
	err := game.ServeWs(room, true, name, c.Response(), c.Request())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, "Created room "+roomID)
}

func joinRoom(c echo.Context) error {
	// Prevent user to join finished room
	deleteFinishedRooms()

	// Check if room exists
	roomID := c.QueryParam("room")
	if _, ok := rooms[roomID]; !ok {
		return c.String(http.StatusNotFound, "Room "+roomID+" not found")
	}

	name := s.TrimSpace(c.QueryParam("name"))
	room := rooms[roomID]
	err := game.ServeWs(room, false, name, c.Response(), c.Request())
	if err != nil {
		// Most probably non unique name used
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, "Joined room "+roomID)
}

func index(c echo.Context) error {
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")

	return c.String(http.StatusOK, "Hello, Tojvi!")
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	e := echo.New()

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339_nano}", "method":"${method}", "uri":"${uri}",` +
			` "status":"${status}", "remote_ip":"${remote_ip}", "latency":"${latency_human}",` +
			` "user_agent":"${user_agent}"}` + "\n",
	}))
	e.Use(middleware.Recover())

	e.Static("/", "./public")

	e.GET("/", index)
	e.GET("/new", createRoom)
	e.GET("/join", joinRoom)

	port := os.Getenv("PORT")
	if port == "" {
		log.Info().Msg("No port specified, defaulting to 8080")
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

func fetchOpenTDBQuestions() []*game.Question {
	openTdb := &data.Provider{
		Name: "OpenTDB",
		Path: "https://opentdb.com/api.php?amount=15&category=9&type=multiple",
		Type: data.MultipleChoice,
		Key:  "",
	}

	c := http.Client{Timeout: time.Second * 10}

	return openTdb.FetchQuestions(c)
}

// TODO: This does not work?
func deleteFinishedRooms() {
	for roomID, room := range rooms {
		if !room.Active {
			log.Debug().Msg("Deleted room [" + roomID + "]")
			delete(rooms, roomID)
		} else {
			log.Debug().Msg("Room [" + roomID + "]: Active")
		}
	}
}

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())

	var letters = []rune("ABCDEFGHJKLMNPQRSTUVWXYZ123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
