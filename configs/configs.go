package configs

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
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
	ApiVersion       int            `yaml:"api_version"`
	Server           Server         `yaml:"server"`
	KeystoreFilepath string         `yaml:"keystore_filepath"`
	HomeServer       string         `yaml:"homeserver"`
	HomeServerDomain string         `yaml:"homeserver_domain"`
	Bridges          []BridgeConfig `yaml:"bridges"`
	RabbitMQ         RabbitMQ       `yaml:"rabbitmq"`
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

func (c *Conf) GetBridgeConfig(name string) (*BridgeConfig, bool) {
	for _, bridge := range c.Bridges {
		if bridge.Name == name {
			return &bridge, true
		}
	}
	return nil, false
}

func ParseImage(client *mautrix.Client, url string) ([]byte, error) {
	fmt.Printf(">>\tParsing image for: %v\n", url)
	contentUrl, err := id.ParseContentURI(url)
	if err != nil {
		return nil, err
	}
	return client.DownloadBytes(context.Background(), contentUrl)
}

func (c *Conf) CheckSuccessPattern(bridgeType string, input string) (bool, error) {
	config, ok := c.GetBridgeConfig(bridgeType)
	if !ok {
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
	config, ok := c.GetBridgeConfig(bridgeType)
	if !ok {
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

func (c *Conf) CheckUserBridgeBotTemplate(bridgeType string, username string) (bool, error) {
	config, ok := c.GetBridgeConfig(bridgeType)
	if !ok {
		return false, fmt.Errorf("bridge type %s not found in configuration", bridgeType)
	}

	if config.UsernameTemplate == "" {
		return false, fmt.Errorf("username template not found for bridge type %s", bridgeType)
	}

	// Convert template pattern to regex pattern
	// Replace {{.}} with .* to match any characters
	regexPattern := strings.ReplaceAll(config.UsernameTemplate, "{{.}}", ".*")
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

func (c *Conf) FormatUsername(bridgeName, username string) (string, error) {
	config, ok := c.GetBridgeConfig(bridgeName)
	if !ok {
		return "", fmt.Errorf("bridge type %s not found in configuration", bridgeName)
	}

	if config.UsernameTemplate == "" {
		return "", fmt.Errorf("username template not found for bridge type %s", bridgeName)
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
		formattedUsername = formattedUsername + ":" + c.HomeServerDomain
	}

	return formattedUsername, nil
}
