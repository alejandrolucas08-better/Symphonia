package call

import "time"

type Participant struct {
	ID       string
	CallID   string
	UserID   string
	Language string
	JoinedAt time.Time
	LeftAt   *time.Time
}
