package models

type Promotion struct {
	ID       int
	Name     string
	Value    string
	ImageURL string
}

type UserClaim struct {
	UserID int64
}

type AdminState struct {
	UserID int64
	State  string
	Data   string
}
