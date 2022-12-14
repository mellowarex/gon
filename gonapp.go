package gon

import (
	"net/http"
	"time"
	"fmt"
	"os"
	"crypto/tls"
	"github.com/mellowarex/gon/logs"
	"github.com/mellowarex/gon/servers"
	"golang.org/x/crypto/acme/autocert"
)

type HServer struct {
	Handler *Multiplexer
	Server 	*http.Server
	Config  *Config
}

var (
	GonApp *HServer
)

func init() {
	GonApp = NewHServer()
}

func NewHServer() *HServer {
	gon := &HServer{
		Handler: InitMux(),
		Server: &http.Server{},
		Config: GConfig,
	}
	return gon
}

// Run start gon web app
func (this *HServer) Run() {
	this.Server.Handler = this.Handler
	this.Server.ReadTimeout = time.Duration(this.Config.Listen.ServerTimeOut) * time.Second
	this.Server.WriteTimeout = time.Duration(this.Config.Listen.ServerTimeOut) * time.Second

	addr, httpsAddr := this.Config.Listen.HTTPAddr, this.Config.Listen.HTTPSAddr

	//prep addr format localhost:port
	if this.Config.Listen.HTTPPort != 0 {
		addr += fmt.Sprintf(":%d", this.Config.Listen.HTTPPort)
	}
	if this.Config.Listen.HTTPSPort != 0 {
		httpsAddr += fmt.Sprintf(":%d", this.Config.Listen.HTTPSPort)
	}

	endRunning := make(chan bool, 1)

	// start server gracefully
	// if https is set in config run in https
	if this.Config.Listen.EnableHTTPS || this.Config.Listen.EnableMutualHTTPS {
		this.Server.Addr = httpsAddr
		go func() {
			time.Sleep(1000 * time.Microsecond)
			server := servers.NewServer(httpsAddr, this.Server.Handler)
			server.Server.ReadTimeout = this.Server.ReadTimeout
			server.Server.WriteTimeout = this.Server.WriteTimeout
			server.ServerName = this.Config.ServerName
			server.AppName = this.Config.AppName
			server.EnvMode = this.Config.EnvMode

			if this.Config.Listen.EnableMutualHTTPS {
				if err := server.ListenAndServeMutualTLS(this.Config.Listen.HTTPSCertFile, this.Config.Listen.HTTPSKeyFile, this.Config.Listen.TrustCaFile); err != nil {
					logs.Critical("Server MutualTLS failed to start, reason: ", err, fmt.Sprintf("pid: %d", os.Getpid()))
					time.Sleep(100 * time.Microsecond)
				}
			} else {
				if this.Config.Listen.AutoTLS {
					m := autocert.Manager{
						Prompt:   	autocert.AcceptTOS,
						HostPolicy:	autocert.HostWhitelist(this.Config.Listen.Domains...),
						Cache:			autocert.DirCache(this.Config.Listen.TLSCacheDir),
					}
					this.Server.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
					this.Config.Listen.HTTPSCertFile, this.Config.Listen.HTTPSKeyFile = "", ""
				}
				if err := server.ListenAndServeTLS(this.Config.Listen.HTTPSCertFile, this.Config.Listen.HTTPSKeyFile); err != nil {
					logs.Critical("Server TLS failed to start, reason: ", err, fmt.Sprintf("pid: %d", os.Getpid()))
					time.Sleep(100 * time.Microsecond)
				}
			}

			endRunning <- true
		}()
	}else {
		this.Server.Addr = addr
		go func() {
			server := servers.NewServer(addr, this.Server.Handler)
			server.Server.ReadTimeout = this.Server.ReadTimeout
			server.Server.WriteTimeout = this.Server.WriteTimeout
			server.ServerName = this.Config.ServerName
			server.AppName = this.Config.AppName
			server.EnvMode = this.Config.EnvMode
			if this.Config.Listen.ListenTCP4 {
				server.Network = "tcp4"
			}
			if err := server.ListenAndServe(); err != nil {
				logs.Critical("Server failed to start, reason: ", err, fmt.Sprintf("pid: %d", os.Getpid()))
				time.Sleep(100 * time.Microsecond)
			}
			endRunning <- true
		}()
	}
	<-endRunning
}