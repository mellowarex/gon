package gon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"errors"

	"github.com/mellocraft/gon/logs"
	"github.com/mellocraft/gon/session"
	"github.com/mellocraft/gon/utils"
	"github.com/mellocraft/gon/context"
)

const (
	DEV = "development"
	PROD = "production"
)

type Config struct {
	AppName 								string
	EnvMode									string // Web App environment: development, production
	RouterCaseSensitive			bool
	ServerName							string
	CopyRequestBody					bool
	EnableGzip							bool
	// MaxMemory and MaxUploadSize are used to limit the request body
	// if the request is not uploading file, MaxMemory is the max size of request body
	// if the request is uploading file, MaxUploadSize is the max size of request body
	MaxMemory           		int64
	MaxUploadSize       		int64
	// EnableErrorsShow    bool
	// EnableErrorsRender  bool
	RecoverPanic						bool
	RecoverFunc			func(*context.Context, *Config)			

	DirectoryIndex						bool
	StaticDir									map[string]string
	StaticExtensionsToGzip		[]string
	StaticCacheFileSize				int
	StaticCacheFileNum				int
	TemplateLeft							string
	TemplateRight							string
	ViewsPath									string

	EnvConfig
}

var (
	// GConfig server & app configuration
	GConfig *Config
	// EnvConfig environment configuration
	EnvConf *EnvConfig
	// AppPath is the absolute path to app
	AppPath string
	// GlobalSessions is instance for session manager
	GlobalSessions *session.Manager
	// appConfigPath is the path to application config
	appConfigPath string
	// envConfigPath is the path to environment config
	envConfigPath string
	// ErrAbort custom error user error induced error
	ErrAbort = errors.New("user stop run")
)

func init() {
	var err error
	AppPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	appConfigPath = filepath.Join(AppPath, "config", "application.json")
	if !utils.FileExists(appConfigPath) { // if config file not found use gon defaults
		logs.Warn("failed to find application.json config in %s, using default config\n", filepath.Join(AppPath, "config"))
		GConfig = initGConfig() //configure with development settings
	} else {
		err = utils.ParseConfigFile(appConfigPath, &GConfig)
		if err != nil {
			panic(err)
		}
	}
	var filename = "development.json" // Fetch config/[production|development].json config by default
	// select production.json if set in ENV VAR
	// ENV var overrides which config to use for web app
	if os.Getenv("GON_ENV") != "" {
		filename = os.Getenv("GON_ENV") + ".json"
	}
	envConfigPath = filepath.Join(AppPath, "config", "environments", filename)
	if !utils.FileExists(envConfigPath) {
		logs.Warn("failed to find environment.json config in %s, using default dev config\n", filepath.Join(AppPath, "config", "environments"))
		EnvConf = initEnvConfig() 
	} else {
		err = utils.ParseConfigFile(envConfigPath, &EnvConf)
		if err != nil {
			panic(err)
		}
	}
	GConfig.ServerName = VERSION + "["+ VERSION_NAME + "]"
	GConfig.Listen = EnvConf.Listen
	GConfig.WebConfig = EnvConf.WebConfig
	GConfig.Log 				= EnvConf.Log
	GConfig.RecoverFunc = defaultRecoverPanic
	initializeLogs(GConfig.EnvMode)
}

// InitGConfig sets GConfig default
func initGConfig() *Config {
	conf := &Config{
		AppName:             "gon",
		EnvMode:             "development",
		RouterCaseSensitive: true,
		ServerName:          VERSION + "[" + VERSION_NAME + "]",
		RecoverPanic:        true,

		CopyRequestBody:    false,
		EnableGzip:         false,
		MaxMemory:          1 << 26, // 64MB
		MaxUploadSize:      1 << 30, // 1GB
	}

	return conf
}

func initializeLogs(envMode string) {
	logs.Reset()
	
	err :=  logs.SetLogger("console", "")
	// logs = logs.Async(100)
	if err != nil {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s with the config %q got err:%s", "console", "", err.Error()))
	}
	config :=  `{
		"filename":"hunter.log",
			"maxlines":10000,
			"maxfiles":88,
			"maxsize":5120,
			"daily":true,
			"maxdays":15,
			"hourly": false,
			"rotate":true,
			"perm":"0600",
			"dir": "` + envMode +`"
	}`
	err = logs.SetLogger("file", config)
	if err != nil {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s with the config %q got err:%s", "file", "", err.Error()))
	}
	logs.SetLogFuncCall(true)
}

func defaultRecoverPanic(ctx *context.Context, cfg *Config) {
	if err := recover(); err != nil {
		if err == ErrAbort {
			return
		}
		if _, ok := ErrorMaps[fmt.Sprint(err)]; ok {
			exception(fmt.Sprint(err), ctx)
			return
		}

		var stack string
		logs.Critical("the request url is ", ctx.Input.URL())
		logs.Critical("Handler crashed with error ", err)
		for i := 1; ;i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			logs.Critical(fmt.Sprintf("%s:%d", file, line))
			stack = stack + fmt.Sprintln(fmt.Sprintf("%s:%d", file, line))
		}
		if cfg.EnvMode == DEV {
			showErr(err, ctx, stack)
		}
		if ctx.Output.Status != 0 {
			ctx.ResponseWriter.WriteHeader(ctx.Output.Status)
		} else {
			ctx.ResponseWriter.WriteHeader(500)
		}
	}
}