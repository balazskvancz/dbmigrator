package dbmigrator

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Database            string `json:"database"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	DriverName          string `json:"driverName"`
	MigrationsTableName string `json:"migrationsTableName"`
	MigrationsFilePath  string `json:"migrationsFilePath"`
}

func loadJsonConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	conf := &Config{}
	if err := json.NewDecoder(file).Decode(conf); err != nil {
		return nil, err
	}

	return conf, nil
}

func loadFromEnv() (*Config, error) {
	var (
		host                = os.Getenv("HOST")
		port                = os.Getenv("PORT")
		database            = os.Getenv("DATABASE")
		username            = os.Getenv("USERNAME")
		password            = os.Getenv("PASSWORD")
		drivername          = os.Getenv("DRIVER_NAME")
		migrationsTableName = os.Getenv("MIGRATIONS_TABLE_NAME")
		migrationsFilePath  = os.Getenv("MIGRATIONS_FILE_PATH")
	)

	cPort, _ := strconv.Atoi(port)

	return &Config{
		Host:                host,
		Port:                cPort,
		Database:            database,
		Username:            username,
		Password:            password,
		DriverName:          drivername,
		MigrationsTableName: migrationsTableName,
		MigrationsFilePath:  migrationsFilePath,
	}, nil
}
