package models

type Promotion struct {
	ID      int
	Name    string
	URL     string
	Deleted bool
}

type UserClaim struct {
	UserID int64
}

type AdminState struct {
	UserID int64
	State  string
	Data   string
}
