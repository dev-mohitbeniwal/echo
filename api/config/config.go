// api/config/config.go
package config

import (
	"log"

	"github.com/spf13/viper"
)

// Configuration stores all the configurations
type Configuration struct {
	Server        ServerConfiguration
	Neo4j         DatabaseConfiguration
	Redis         RedisConfiguration
	Elasticsearch ElasticsearchConfiguration
}

// ServerConfiguration stores the port and other web server settings
type ServerConfiguration struct {
	Port string
}

// DatabaseConfiguration stores data for database connection
type DatabaseConfiguration struct {
	URI string
}

// RedisConfiguration stores data for Redis connection
type RedisConfiguration struct {
	Addr            string
	DefaultCacheTTL string
}

// ElasticsearchConfiguration stores data for Elasticsearch connection
type ElasticsearchConfiguration struct {
	URL string
}

var config *Configuration

func InitConfig() error {
	viper.AddConfigPath("config") // path to look for the config file in
	viper.SetConfigName("config") // name of the config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name

	viper.AutomaticEnv() // read in environment variables that match

	// Set default configurations
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("neo4j.uri", "bolt://localhost:7687")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("elasticsearch.url", "http://localhost:9200")
	viper.SetDefault("redis.defaultCacheTTL", "10m")
	viper.SetDefault("log.file", "logging/api.log")

	// Attempt to read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No config file found. Using default settings and environment variables.")
		} else {
			return err
		}
	}

	// Unmarshal the configuration into the Configuration struct
	err := viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	return nil
}

// GetConfig returns the loaded configuration
func GetConfig() *Configuration {
	return config
}

// GetString retrieves a string value from the configuration
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt retrieves an integer value from the configuration
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool retrieves a boolean value from the configuration
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetFloat64 retrieves a float64 value from the configuration
func GetFloat64(key string) float64 {
	return viper.GetFloat64(key)
}
