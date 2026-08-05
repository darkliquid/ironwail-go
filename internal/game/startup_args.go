package game

type startupOptions struct {
	BaseDir    string
	GameDir    string
	Dedicated  bool
	Listen     bool
	MaxClients int
	Port       int
	Args       []string
}

type StartupOptions = startupOptions
