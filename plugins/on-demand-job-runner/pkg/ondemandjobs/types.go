package ondemandjobs

type Config struct {
	Organization      string `mapstructure:"organization"`
	Cluster           string `mapstructure:"cluster"`
	Token             string `mapstructure:"token"`
	Host              string `mapstructure:"host"`
	MaxConcurrentJobs int    `mapstructure:"maxConcurrentJobs"`
	DevMode           bool   `mapstructure:"devMode"`
}
