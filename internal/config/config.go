package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Env      string `yaml:"env" env-default:"local"`
	LogFile  string `yaml:"log_file" env:"LOG_FILE"`
	LogLevel string `yaml:"log_level" env-default:"info" env:"LOG_LEVEL"`

	Auth     AuthConfig     `yaml:"auth"`
	GRPCAddr GRPCConfig     `yaml:"grpc"`
	Postgres DatabaseConfig `yaml:"postgres" env-required:"true"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port" env:"GRPC_PORT"`
	Timeout time.Duration `yaml:"timeout" env:"GRPC_TIMEOUT"`
}

type AuthConfig struct {
	JWT JWTVerifyConfig `yaml:"jwt"`
}

type DatabaseConfig struct {
	Host        string        `yaml:"host" env:"PG_HOST" env-required:"true"`
	Port        int           `yaml:"port" env:"PG_PORT" env-required:"true"`
	User        string        `yaml:"user" env:"PG_USER" env-required:"true"`
	Password    string        `yaml:"password" env:"PG_PASSWORD" env-required:"true"`
	DBName      string        `yaml:"DBName" env:"PG_DBNAME" env-required:"true"`
	ConnectConf ConnectConfig `yaml:"connect_conf"`
}

type ConnectConfig struct {

	// Максимум соединений в пуле
	MaxConns int32 `yaml:"max_conns" env:"PG_MAX_CONNS"`

	// Минимум соединений, которые пул будет стараться держать открытыми
	MinConns int32 `yaml:"min_conns" env:"PG_MIN_CONNS"`

	// Максимальная продолжительность жизни соединения
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"PG_MAX_CONN_LIFETIME"`

	// Максимальное время простоя соединения.
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time" env:"PG_MAX_CONN_IDLE_TIME"`
}

type JWTVerifyConfig struct {
	Issuer     string        `yaml:"issuer" env:"JWT_ISSUER"`
	SigningKey string        `yaml:"signing_key" env:"JWT_SIGNING_KEY"`
	ClockSkew  time.Duration `yaml:"clock_skew" env:"JWT_CLOCK_SKEW"`
}

func MustLoad(name string) *Config {
	path := os.Getenv(name)

	if path == "" {
		panic("путь в конфигу пуст")
	}

	return MustLoadPath(path)
}

func MustLoadPath(configPath string) *Config {
	// проверяем существует ли файл
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("конфиг файл не существует: " + configPath)
	}

	// Читаем файл полностью в память
	raw, err := os.ReadFile(configPath)
	if err != nil {
		panic("не удалось прочитать файл конфига: " + err.Error())
	}

	// Расширяем в нeм все ${VARS}, os.Getenv("VARS") или "" если не задана
	expanded := os.ExpandEnv(string(raw))

	// Декодируем развeрнутый YAML
	var cfg Config
	if err = yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		panic("не получилось распарсить YAML: " + err.Error())
	}

	// читаем env теги
	if err = cleanenv.ReadEnv(&cfg); err != nil {
		panic("не получилось прочитать конфиг: " + err.Error())
	}

	return &cfg
}
