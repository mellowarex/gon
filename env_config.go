package gon

import "crypto/tls"

type EnvConfig struct {
	Listen 			Listen
	WebConfig 	WebConfig
	Log       	Log
}

// Listen: http and https related config
type Listen struct {
	ServerTimeOut     int64
	ListenTCP4        bool

	Domains           []string

	EnableHTTP        bool
	HTTPAddr          string
	HTTPPort          int

	TLSCacheDir       string
	AutoTLS           bool
	EnableHTTPS       bool
	EnableMutualHTTPS bool
	HTTPSAddr         string
	HTTPSPort         int
	HTTPSCertFile     string
	HTTPSKeyFile      string
	TrustCaFile       string

	ClientAuth        int
}

// WebConfig holds web related config
type WebConfig struct {
	FlashName              string
	FlashSeparator         string

	EnableXSRF             bool
	XSRFKey                string
	XSRFExpire             int
	Session                SessionConfig
}

// SessionConfig holds session related config
type SessionConfig struct {
	SessionOn                    bool
	SessionName                  string
	SessionGCMaxLifetime         int64
	SessionProviderConfig        string
	SessionCookieLifeTime        int
	SessionAutoSetCookie         bool
	SessionDisableHTTPOnly       bool // used to allow for cross domain cookies/javascript cookies.
	SessionEnableSidInHTTPHeader bool // enable store/get the sessionId into/from http headers
	SessionNameInHTTPHeader      string
	SessionEnableSidInURLQuery   bool // enable get the sessionId from Url Query params
}

// LogConfig holds Log related config
type Log struct {
	DateLog          		bool
	AccessLogs  			 	bool
	EnableStaticLogs 		bool   //log static files requests default: false
	AccessLogsFormat 		string //access log format: JSON_FORMAT, APACHE_FORMAT or empty string
	LMail             	LogMail
}

type LogMail struct {
	SendMail   		bool
	Env  					string
	Username			string
	Password  		string
	Host  				string
	Subject  			string
	FromAddress   string
	SendTo        []string
	Level  				int
	HttpStatus    map[string]bool
}

// InitEnvConfig default environment configuration
func initEnvConfig() *EnvConfig {
	envConf := &EnvConfig{
		Listen: Listen{
			ServerTimeOut: 15,
			ListenTCP4:    false,
			EnableHTTP:    true,
			AutoTLS:       false,
			Domains:       []string{},
			TLSCacheDir:   ".",
			HTTPAddr:      "",
			HTTPPort:      7000,
			EnableHTTPS:   false,
			HTTPSAddr:     "",
			HTTPSPort:     10443,
			HTTPSCertFile: "",
			HTTPSKeyFile:  "",
			ClientAuth:    int(tls.RequireAndVerifyClientCert),
		},
		WebConfig: WebConfig{
			FlashName:              "GON_FLASH",
			FlashSeparator:         "GONFLASH",
			EnableXSRF:             false,
			XSRFKey:                "gonxsrf",
			XSRFExpire:             0,
			Session: SessionConfig{
				SessionOn:                    false,
				SessionName:                  "gonsession",
				SessionGCMaxLifetime:         3600,
				SessionProviderConfig:        "",
				SessionDisableHTTPOnly:       false,
				SessionCookieLifeTime:        3600, // set cookie default is the browser life
				SessionAutoSetCookie:         true,
				SessionEnableSidInHTTPHeader: false, // enable store/get the sessionId into/from http headers
				SessionNameInHTTPHeader:      "Gonsession",
				SessionEnableSidInURLQuery:   false, // enable get the sessionId from Url Query params
			},
		},
		Log: Log{
			DateLog:          true,
			EnableStaticLogs: true,
			AccessLogs: 				true,
			AccessLogsFormat: "APACHE_FORMAT",
		},
	}
	return envConf
}
