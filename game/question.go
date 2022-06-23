package game

type Question struct {
	Type          string
	Category      string
	Description   string
	Choices       []string
	CorrectChoice string
	Answers       map[*Player]int
	Reward        int
}

type JSONQuestion struct {
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Choices       []string `json:"choices"`
	CorrectChoice string   `json:"correct_choice"`
	Reward        int      `json:"reward"`
}

func (q *Question) ToJSONQuestion() *JSONQuestion {
	jsonQuestion := &JSONQuestion{Type: q.Type, Description: q.Description, Choices: q.Choices, CorrectChoice: q.CorrectChoice, Reward: q.Reward}

	return jsonQuestion
}

func (q *Question) AwardScores() {
	oneVoters := []*Player{}
	twoVoters := []*Player{}
	for player, vote := range q.Answers {
		if vote == 1 {
			oneVoters = append(oneVoters, player)
		} else if vote == 2 {
			twoVoters = append(twoVoters, player)
		}
	}
	if len(oneVoters) < len(twoVoters) {
		for _, player := range oneVoters {
			player.Score += q.Reward
		}
	} else if len(oneVoters) > len(twoVoters) {
		for _, player := range twoVoters {
			player.Score += q.Reward
		}
		// Tie
	} else {
		for player := range q.Answers {
			player.Score += q.Reward / 2
		}
	}
}
