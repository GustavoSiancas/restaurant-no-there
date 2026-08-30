package database

// Config contains the PostgreSQL connection settings.
type Config struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
	SSLMode  string
}

func (c Config) DSN() string {
	return "host=" + c.Host + " port=" + c.Port + " dbname=" + c.Database +
		" user=" + c.User + " password=" + c.Password + " sslmode=" + c.SSLMode
}
