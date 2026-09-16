package call

import "time"

type Call struct {
	ID        string
	Code      string
	Name      string
	Status    string
	CreatedBy string
	CreatedAt time.Time
	EndedAt   *time.Time
}
