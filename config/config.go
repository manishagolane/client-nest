package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var CONFIG_PATH_MAP = map[string]string{
	"development": "./config/development.json",
	"staging":     "./config/staging.json",
	"production":  "./config/production.json",
}

func Get(key string) interface{} {
	return viper.Get(key)
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetBoolean(key string) bool {
	return viper.GetBool(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
}

func GetMap(key string) map[string]interface{} {
	return viper.GetStringMap(key)
}

func GetSlice(key string) []string {
	return viper.GetStringSlice(key)
}

func formatEnvKeys(envData []byte) []byte {
	formattedEnv := make([]byte, 0, len(envData))
	data := strings.Split(string(envData), "\n")
	for _, line := range data {
		if line == "" {
			continue
		}
		splits := strings.SplitN(line, "=", 2)
		newKKey := strings.ReplaceAll(splits[0], "_", ".")
		formattedEnv = append(formattedEnv, []byte(newKKey+"="+splits[1]+"\n")...)
	}
	return formattedEnv
}

func Init() {
	runtimeEnv := flag.String("env", "development", "runtime environment")
	flag.Parse()

	fmt.Println("Runtime Environment:", *runtimeEnv)

	path, ok := CONFIG_PATH_MAP[*runtimeEnv]
	if !ok {
		panic("Invalid runtime environment")
	}
	fmt.Println("Config Path:", path)

	viper.SetConfigType("json")
	reader, err := os.Open(path)
	if err != nil {
		panic(fmt.Errorf("unable to read config file\n %w", err))
	}
	err = viper.ReadConfig(reader)
	if err != nil {
		panic(fmt.Errorf("unable to read config file\n %w", err))
	}
	reader, err = os.Open(".env")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		panic(fmt.Errorf("unable to read env config file\n %w", err))
	}
	fmt.Printf("Redis Config: %+v\n", viper.Get("redis"))
	defer reader.Close()
	viper.SetConfigType("env")
	envData, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("unable to read config file data\n %w", err))
	}
	envData = formatEnvKeys(envData)
	viper.MergeConfig(bytes.NewBuffer(envData))
}
