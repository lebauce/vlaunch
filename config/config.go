package config

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/kardianos/osext"
	"github.com/spf13/viper"
)

var cfg *viper.Viper

func InitConfig(cfgFiles []string) error {
	cfg = viper.New()
	cfg.SetConfigType("yaml")
	cfg.SetDefault("distro_type", "Linux_64")
	cfg.SetDefault("disk_type", "raw")

	for _, path := range cfgFiles {
		configFile, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := cfg.MergeConfig(configFile); err != nil {
			return err
		}
	}

	replacer := strings.NewReplacer(".", "_", "-", "_")
	cfg.SetEnvPrefix("VLAUNCH")
	cfg.SetEnvKeyReplacer(replacer)
	cfg.AutomaticEnv()

	dataPath := cfg.GetString("data_path")
	if dataPath == "" {
		if executableFolder, err := osext.ExecutableFolder(); err == nil {
			dataPath = path.Join(executableFolder, ".vlaunch")
			if err := os.MkdirAll(dataPath, 0755); err != nil {
				return err
			}
			cfg.Set("data_path", dataPath)
		}
	}

	log.Printf("Using %s as data path\n", dataPath)
	os.Setenv("VBOX_USER_HOME", dataPath)
	return nil
}

func GetConfig() *viper.Viper {
	return cfg
}
