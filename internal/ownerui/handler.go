package ownerui

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

const (
	OwnerOnlyCopy   = "Bot sedang dalam mode riset pribadi owner."
	HomeCopy        = "RESEARCH ONLY — owner research shell aktif. Trading, Buy, Sell, Confirm, dan Auto belum tersedia."
	ResearchCopy    = "Research — analisis belum diaktifkan. P1 hanya menyiapkan shell read-only."
	WatchlistCopy   = "Watchlist — belum ada kandidat aktif. P1 masih foundation."
	SkillCopy       = "Skill — baseline v1.1.0 tercatat sebagai research-only; activation belum tersedia."
	RunsCopy        = "Runs — durable research runner belum tersedia sampai P2.3."
	SystemCopy      = "System — mode RESEARCH ONLY. AI provider, wallet key, signing, dan real order tidak aktif."
	PauseCopy       = "Pause — execution belum aktif; sistem tetap fail-closed / NO TRADE."
	UnsupportedCopy = "RESEARCH ONLY — perintah trading dan exchange legacy dibekukan. Gunakan menu Research."
)

type Decision struct {
	Allowed        bool
	Text           string
	Keyboard       *models.ReplyKeyboardMarkup
	Mutates        bool
	TradingEnabled bool
}

func Decide(input string, isOwner, isPrivate bool) Decision {
	if !isPrivate {
		return Decision{}
	}
	if !isOwner {
		return Decision{Text: OwnerOnlyCopy}
	}
	text := strings.ToLower(strings.TrimSpace(input))
	copy := UnsupportedCopy
	switch text {
	case "/start", "start", "/menu", "menu":
		copy = HomeCopy
	case "/research", "research":
		copy = ResearchCopy
	case "/watchlist", "watchlist":
		copy = WatchlistCopy
	case "/skill", "skill":
		copy = SkillCopy
	case "/runs", "runs":
		copy = RunsCopy
	case "/system", "system":
		copy = SystemCopy
	case "/pause", "pause":
		copy = PauseCopy
	}
	return Decision{Allowed: true, Text: copy, Keyboard: Keyboard()}
}
