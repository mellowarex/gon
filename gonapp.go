package gon

import (
	"net/http"
	"time"
	"fmt"

	"github.com/mellowarex/gon/servers"
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

			}
			endRunning <- true
		}()
	}
	<-endRunning
}