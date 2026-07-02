package accounts

type LocalAccount struct {
	Username    string `json:"username"`
	SID         string `json:"sid"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	LastLogon   string `json:"last_logon"`
	IsAdmin     bool   `json:"is_admin"`
}

type LocalGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}