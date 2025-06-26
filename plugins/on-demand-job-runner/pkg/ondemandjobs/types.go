package ondemandjobs

type Config struct {
	Organization string `mapstructure:"organization"`
	Cluster      string `mapstructure:"cluster"`
	Token        string `mapstructure:"token"`
	Host         string `mapstructure:"host"`
	DevMode      bool   `mapstructure:"devMode"`
}
