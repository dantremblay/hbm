package types

type Config struct {
	AppPath              string
	Username             string
	DisableOwnershipCheck bool
}

type AllowResult struct {
	Allow bool
	Msg   map[string]string
	Error string
}
