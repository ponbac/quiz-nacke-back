package data

import (
	"encoding/json"
	"html"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/ponbac/majority-wins/game"
	"github.com/rs/zerolog/log"
)

type QuestionProvider interface {
	FetchQuestions(client http.Client) []*game.Question
}

type TTAProvider struct {
	Name string
	Path string
	Type QuestionType
	Key  string
}

var TtaProvider = &TTAProvider{
	Name: "Trivia",
	Path: "https://the-trivia-api.com/api/questions?limit=20",
	Type: None,
	Key:  "Trivia",
}

type ttaQuestion struct {
	Category         string   `json:"category"`
	Type             string   `json:"type"`
	Difficulty       string   `json:"difficulty"`
	Question         string   `json:"question"`
	CorrectAnswer    string   `json:"correctAnswer"`
	IncorrectAnswers []string `json:"incorrectAnswers"`
}

func (p *TTAProvider) FetchQuestions() []*game.Question {
	// Make request
	c := http.Client{Timeout: time.Second * 10}
	resp, err := c.Get(p.Path)
	if err != nil {
		log.Error().Err(err).Msg("Could not fetch questions from " + p.Path)
	}
	defer resp.Body.Close()

	// Read response in bytes
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err)
	}

	// Parse response to object
	var pQuestions []*ttaQuestion
	err = json.Unmarshal(bodyBytes, &pQuestions)
	if err != nil {
		log.Error().Err(err)
	}

	// Convert to game.Question
	var questions []*game.Question
	for _, pQuestion := range pQuestions {
		questions = append(questions, pQuestion.toQuestion())
	}

	log.Debug().Msgf("Fetched %d questions from %s", len(questions), p.Name)
	return questions
}

func (q *ttaQuestion) toQuestion() *game.Question {
	// html unescape question and choices
	q.Question = html.UnescapeString(q.Question)
	q.CorrectAnswer = html.UnescapeString(q.CorrectAnswer)
	for i, incorrectAnswer := range q.IncorrectAnswers {
		q.IncorrectAnswers[i] = html.UnescapeString(incorrectAnswer)
	}

	// concat and shuffle choices
	allChoices := append(q.IncorrectAnswers, q.CorrectAnswer)
	rand.Seed(time.Now().UnixNano())
	for i := len(allChoices) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		allChoices[i], allChoices[j] = allChoices[j], allChoices[i]
	}

	// calculate reward
	reward := 1
	if q.Difficulty == "medium" {
		reward = 2
	} else if q.Difficulty == "hard" {
		reward = 3
	}
	if q.Type == "boolean" {
		reward -= 1
		if reward < 1 {
			reward = 1
		}
	}

	return &game.Question{
		Category:      q.Category,
		Type:          q.Type,
		Reward:        reward,
		Description:   q.Question,
		CorrectChoice: q.CorrectAnswer,
		Choices:       allChoices,
		Answers:       make(map[*game.Player]int),
	}
}
