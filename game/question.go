package game

type Question struct {
	Type             string
	Category         string
	Description      string
	Choices          []string
	CorrectChoice    string
	Answers          map[*Player]int
	Reward           int
	CorrectPlayers   []*Player
	IncorrectPlayers []*Player
}

type JSONQuestion struct {
	Type             string   `json:"type"`
	Description      string   `json:"description"`
	Choices          []string `json:"choices"`
	CorrectChoice    string   `json:"correct_choice"`
	Reward           int      `json:"reward"`
	Answers          []string `json:"answers"`
	CorrectPlayers   []string `json:"correct_players"`
	IncorrectPlayers []string `json:"incorrect_players"`
}

func (q *Question) ToJSONQuestion() *JSONQuestion {
	correctPlayerNames := make([]string, len(q.CorrectPlayers))
	for i, player := range q.CorrectPlayers {
		correctPlayerNames[i] = player.Name
	}
	incorrectPlayerNames := make([]string, len(q.IncorrectPlayers))
	for i, player := range q.IncorrectPlayers {
		incorrectPlayerNames[i] = player.Name
	}
	answers := make([]string, len(q.Answers))
	for p := range q.Answers {
		answers = append(answers, p.Name)
	}

	return &JSONQuestion{Type: q.Type, Description: q.Description, Choices: q.Choices, CorrectChoice: q.CorrectChoice, Reward: q.Reward, Answers: answers, CorrectPlayers: correctPlayerNames, IncorrectPlayers: incorrectPlayerNames}
}

func (q *Question) AwardScores() {
	//log.Debug().Msgf("Awarding scores for answer %s", q.CorrectChoice)

	answerIndex := indexOfAnswer(q)
	for player, vote := range q.Answers {
		if vote == answerIndex {
			player.Score += q.Reward * 2
			q.CorrectPlayers = append(q.CorrectPlayers, player)
		} else {
			q.IncorrectPlayers = append(q.IncorrectPlayers, player)
		}
	}
}

func indexOfAnswer(q *Question) int {
	for i, choice := range q.Choices {
		if choice == q.CorrectChoice {
			return i
		}
	}
	return -1
}
