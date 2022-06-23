package data

import (
	"encoding/json"
	"html"
	"io/ioutil"
	"math/rand"
	"net/http"

	"github.com/ponbac/majority-wins/game"
	"github.com/rs/zerolog/log"
)

type QuestionType int

const (
	MultipleChoice QuestionType = iota
	FreeText
)

type Provider struct {
	Name string
	Path string
	Type QuestionType
	Key  string
}

type ProviderResponse struct {
	ResponseCode int `json:"response_code"`
	Results      []*ProviderQuestion
}

type ProviderQuestion struct {
	Category         string   `json:"category"`
	Type             string   `json:"type"`
	Difficulty       string   `json:"difficulty"`
	Question         string   `json:"question"`
	CorrectAnswer    string   `json:"correct_answer"`
	IncorrectAnswers []string `json:"incorrect_answers"`
}

func (p *Provider) FetchQuestions(client http.Client) []*game.Question {
	// Make request
	resp, err := client.Get(p.Path)
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
	var pResponse ProviderResponse
	err = json.Unmarshal(bodyBytes, &pResponse)
	if err != nil {
		log.Error().Err(err)
	}

	// Extract questions
	var pQuestions []*ProviderQuestion
	pQuestions = append(pQuestions, pResponse.Results...)

	// Convert to game.Question
	var questions []*game.Question
	for _, pQuestion := range pQuestions {
		questions = append(questions, pQuestion.ToQuestion())
	}

	log.Debug().Msgf("Fetched %d questions from %s", len(questions), p.Name)
	return questions
}

func (q *ProviderQuestion) ToQuestion() *game.Question {
	// html unescape question and choices
	q.Question = html.UnescapeString(q.Question)
	q.CorrectAnswer = html.UnescapeString(q.CorrectAnswer)
	for i, incorrectAnswer := range q.IncorrectAnswers {
		q.IncorrectAnswers[i] = html.UnescapeString(incorrectAnswer)
	}

	// concat and shuffle choices
	allChoices := append(q.IncorrectAnswers, q.CorrectAnswer)
	for i := len(allChoices) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		allChoices[i], allChoices[j] = allChoices[j], allChoices[i]
	}

	return &game.Question{
		Category:      q.Category,
		Type:          q.Type,
		Reward:        2,
		Description:   q.Question,
		CorrectChoice: q.CorrectAnswer,
		Choices:       allChoices,
		Answers:       make(map[*game.Player]int),
	}
}

// func main() {
// 	openTdb := &Provider{
// 		Name: "OpenTDB",
// 		Path: "https://opentdb.com/api.php?amount=10&category=9&type=multiple",
// 		Type: MultipleChoice,
// 		Key:  "",
// 	}

// 	c := http.Client{Timeout: time.Second * 10}
// 	questions := openTdb.FetchQuestions(c)

// 	for _, question := range questions {
// 		fmt.Println(question)
// 	}
// }
