package ownerui

import "github.com/go-telegram/bot/models"

const (
	BtnResearch  = "Research"
	BtnWatchlist = "Watchlist"
	BtnSkill     = "Skill"
	BtnRuns      = "Runs"
	BtnSystem    = "System"
	BtnPause     = "Pause"
)

func Keyboard() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{{Text: BtnResearch}, {Text: BtnWatchlist}},
			{{Text: BtnSkill}, {Text: BtnRuns}},
			{{Text: BtnSystem}, {Text: BtnPause}},
		},
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}
