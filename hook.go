package gon

import (
	"github.com/mellocraft/gon/session"
	"net/http"
	"path/filepath"
	"sync"
	"os"
)

type hookfunc func() error

var (
	hooks = make([]hookfunc, 0) // store slice of hook func
	initHttp sync.Once
)

func init() {
	initializeServer()
}

// RegisterHook used to register hookfunc
func RegisterHook(hf ...hookfunc) {
	hooks = append(hooks, hf...)
}

func initializeServer() {
	initHttp.Do(func() {
		RegisterHook(
			registerDefaultErrorHandler,
			registerSession,
			)

		for _, hk := range hooks {
			if err := hk(); err != nil {
				panic(err)
			}
		}
	})
}

func registerSession() error {
	if GConfig.WebConfig.Session.SessionOn {
		var err error
		appPath, err := os.Getwd()
		conf := new(session.CookieConfig)
		conf.CookieName = GConfig.WebConfig.Session.SessionName
		conf.EnableSetCookie = GConfig.WebConfig.Session.SessionAutoSetCookie
		conf.Gclifetime = GConfig.WebConfig.Session.SessionGCMaxLifetime
		conf.Secure = GConfig.Listen.EnableHTTPS
		conf.CookieLifeTime = GConfig.WebConfig.Session.SessionCookieLifeTime
		conf.ProviderConfig = filepath.Join(appPath,"config", "cookie" + ".json")
		conf.DisableHTTPOnly = GConfig.WebConfig.Session.SessionDisableHTTPOnly
		// conf.Domain = GConfig.WebConfig.Session.SessionDomain
		conf.EnableSidInHTTPHeader = GConfig.WebConfig.Session.SessionEnableSidInHTTPHeader
		conf.SessionNameInHTTPHeader = GConfig.WebConfig.Session.SessionNameInHTTPHeader
		conf.EnableSidInURLQuery = GConfig.WebConfig.Session.SessionEnableSidInURLQuery

		if GlobalSessions, err = session.NewManager(conf); err != nil {
			return err
		}
		go GlobalSessions.GC()
	}
	return nil
}

// register default error http handlers, 404,401,403,500 and 503.
func registerDefaultErrorHandler() error {
	m := map[string]func(http.ResponseWriter, *http.Request){
		"401": unauthorized,
		"402": paymentRequired,
		"403": forbidden,
		"404": notFound,
		"405": methodNotAllowed,
		"500": internalServerError,
		"501": notImplemented,
		"502": badGateway,
		"503": serviceUnavailable,
		"504": gatewayTimeout,
		"417": invalidxsrf,
		"422": missingxsrf,
		"413": payloadTooLarge,
	}
	for e, h := range m {
		if _, ok := ErrorMaps[e]; !ok {
			ErrorHandler(e, h)
		}
	}
	return nil
}