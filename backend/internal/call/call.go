package call

import "time"

type Language string

const (
	LanguagePortuguese Language = "PT-BR"
	LanguageEnglish    Language = "EN-US"
	LanguageSpanish    Language = "ES-ES"
	LanguageFrench     Language = "FR-FR"
)

type Status string

const (
	StatusWaiting Status = "waiting"
	StatusActive  Status = "active"
	StatusEnded   Status = "ended"
)

type Participant struct {
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`
	SpokenLanguage Language  `json:"spoken_language"`
	HeardLanguage  Language  `json:"heard_language"`
	JoinedAt       time.Time `json:"joined_at"`
}

type Call struct {
	ID           int64         `json:"id"`
	Code         string        `json:"code"`
	HostUserID   int64         `json:"host_user_id"`
	Status       Status        `json:"status"`
	Participants []Participant `json:"participants"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	EndedAt      *time.Time    `json:"ended_at"`
}

func ValidLanguage(language Language) bool {
	switch language {
	case LanguagePortuguese, LanguageEnglish, LanguageSpanish, LanguageFrench:
		return true
	default:
		return false
	}
}
