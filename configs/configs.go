package configs

import (
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	"gopkg.in/yaml.v3"
)

type Tls struct {
	Crt string `yaml:"crt"`
	Key string `yaml:"key"`
}

type RabbitMQ struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Host     string `yaml:"host"`
	IsTLs    bool   `yaml:"is_tls"`
	Tls      Tls    `yaml:"tls"`
}

type Server struct {
	Port string `yaml:"port"`
	Host string `yaml:"host"`
	Tls  Tls    `yaml:"tls"`
}

type BridgeConfig struct {
	Name                    string            `yaml:"name"`
	BotName                 string            `yaml:"botname"`
	UsernameTemplate        string            `yaml:"username_template"`
	DisplayUsernameTemplate string            `yaml:"display_username_template"`
	Cmd                     map[string]string `yaml:"cmd"` // ← map instead of slice of maps
}

type Conf struct {
	ApiVersion              int            `yaml:"api_version"`
	Server                  Server         `yaml:"server"`
	KeystoreFilepath        string         `yaml:"keystore_filepath"`
	HomeServer              string         `yaml:"homeserver"`
	HomeServerDomain        string         `yaml:"homeserver_domain"`
	Bridges                 []BridgeConfig `yaml:"bridges"`
	RabbitMQ                RabbitMQ       `yaml:"rabbitmq"`
	MAS_CLIENT_ID           string         `yaml:"mas_client_id"`
	MAS_CLIENT_SECRET       string         `yaml:"mas_client_secret"`
	API_AUTHENTICATION_INFO string         `yaml:"api_authentication_info"`
	DATABASE_KEY            string         `yaml:"db_key"`
}

func GetConf() (*Conf, error) {
	c := &Conf{}
	yamlFile, err := os.ReadFile("conf.yaml")
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func GetBridgeConfigs() ([]*BridgeConfig, error) {
	cfg, err := GetConf()
	if err != nil {
		debug.PrintStack()
		return nil, err
	}
	var bridgeConfigs []*BridgeConfig
	for _, bridge := range cfg.Bridges {
		bridgeConfigs = append(bridgeConfigs, &bridge)
	}
	return bridgeConfigs, nil
}

func GetBridgeConfigByBotname(botName string) (*BridgeConfig, error) {
	cfg, err := GetConf()
	if err != nil {
		debug.PrintStack()
		return nil, err
	}
	for _, bridge := range cfg.Bridges {
		if bridge.BotName == botName {
			return &bridge, nil
		}
	}
	return nil, nil
}

func GetBridgeConfig(name string) (*BridgeConfig, error) {
	cfg, err := GetConf()
	if err != nil {
		debug.PrintStack()
		return nil, err
	}
	for _, bridge := range cfg.Bridges {
		if bridge.Name == name {
			return &bridge, nil
		}
	}
	return nil, nil
}

// func ParseImage(client *mautrix.Client, url string) ([]byte, error) {
// 	fmt.Printf(">>\tParsing image for: %v\n", url)
// 	contentUrl, err := id.ParseContentURI(url)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return client.Bytes(context.Background(), contentUrl)
// }

func (c *Conf) CheckSuccessPattern(bridgeType string, input string) (bool, error) {
	config, err := GetBridgeConfig(bridgeType)
	if err != nil {
		debug.PrintStack()
		return false, err
	}

	if config == nil {
		return false, fmt.Errorf("bridge type %s not found in configuration", bridgeType)
	}

	successPattern, ok := config.Cmd["success"]
	if !ok {
		return false, fmt.Errorf("success pattern not found for bridge type %s", bridgeType)
	}

	// Replace %s with .* to create a regex pattern
	regexPattern := strings.ReplaceAll(successPattern, "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, input)
	if err != nil {
		return false, fmt.Errorf("error matching pattern: %v", err)
	}

	return matched, nil
}

func (c *Conf) CheckOngoingPattern(bridgeType string, input string) (bool, error) {
	config, err := GetBridgeConfig(bridgeType)
	if err != nil {
		debug.PrintStack()
		return false, err
	}

	if config == nil {
		return false, fmt.Errorf("bridge type %s not found in configuration", bridgeType)
	}

	ongoingPattern, ok := config.Cmd["ongoing"]
	if !ok {
		return false, fmt.Errorf("ongoing pattern not found for bridge type %s", bridgeType)
	}

	matched, err := regexp.MatchString(ongoingPattern, input)
	if err != nil {
		return false, fmt.Errorf("error matching pattern: %v", err)
	}

	return matched, nil
}

func CheckUserBridgeBotTemplate(bridgeConfig BridgeConfig, username string) (bool, error) {
	// Convert template pattern to regex pattern
	// Replace {{.}} with .* to match any characters
	regexPattern := strings.ReplaceAll(bridgeConfig.UsernameTemplate, "{{.}}", ".*")
	// Escape any other special regex characters
	regexPattern = regexp.QuoteMeta(regexPattern)
	// Restore the .* pattern
	regexPattern = strings.ReplaceAll(regexPattern, "\\.\\*", ".*")

	matched, err := regexp.MatchString(regexPattern, username)
	if err != nil {
		return false, fmt.Errorf("error matching username pattern: %v", err)
	}

	return matched, nil
}

func FormatUsername(bridgeName, username string) (*string, error) {
	config, err := GetBridgeConfig(bridgeName)
	if err != nil {
		debug.PrintStack()
		return nil, err
	}

	if config == nil {
		return nil, fmt.Errorf("bridge type %s not found in configuration", bridgeName)
	}

	if config.UsernameTemplate == "" {
		return nil, fmt.Errorf("username template not found for bridge type %s", bridgeName)
	}
	username = strings.ReplaceAll(username, "+", "")

	// Replace {{.}} with the actual username
	formattedUsername := strings.ReplaceAll(config.UsernameTemplate, "{{.}}", username)

	// Ensure the username is properly formatted as a Matrix user ID
	// Matrix user IDs should be in the format @localpart:domain
	if !strings.HasPrefix(formattedUsername, "@") {
		formattedUsername = "@" + formattedUsername
	}
	if !strings.Contains(formattedUsername, ":") {
		cfg, err := GetConf()
		if err != nil {
			debug.PrintStack()
			return nil, err
		}
		formattedUsername = formattedUsername + ":" + cfg.HomeServerDomain
	}

	return &formattedUsername, nil
}
