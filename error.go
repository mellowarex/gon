package gon

import (
	"fmt"
	"github.com/mellocraft/gon/context"
	"html/template"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
)

const (
	errorTypeHandler = iota
	errorTypeController
)


type errorInfo struct {
	controllerType reflect.Type
	handler        http.HandlerFunc
	method         string
	errorType      int
}

// ErrorMaps holds map of http handlers for each error string.
// there are 10 kinds default error(40x and 50x)
var ErrorMaps = make(map[string]*errorInfo, 10)

// show error string as simple text message.
// if error string is empty, show 503 or 500 error as default.
func exception(errCode string, ctx *context.Context) {
	atoi := func(code string) int {
		v, err := strconv.Atoi(code)
		if err == nil {
			return v
		}
		if ctx.Output.Status == 0 {
			return 503
		}
		return ctx.Output.Status
	}

	for _, ec := range []string{errCode, "503", "500"} {
		if h, ok := ErrorMaps[ec]; ok {
			executeError(h, ctx, atoi(ec))
			return
		}
	}
	// if 50x error has been removed from errorMap
	ctx.ResponseWriter.WriteHeader(atoi(errCode))
	ctx.WriteString(errCode)
}

func executeError(err *errorInfo, ctx *context.Context, code int) {
	// make sure to log the error in the access log
	//LogAccess(ctx, nil, code)

	if err.errorType == errorTypeHandler {
		ctx.ResponseWriter.WriteHeader(code)
		err.handler(ctx.ResponseWriter, ctx.Request)
		return
	}
	if err.errorType == errorTypeController {
		ctx.Output.SetStatus(code)
		// Invoke the request handler
		vc := reflect.New(err.controllerType)
		execController, ok := vc.Interface().(ControllerInterface)
		if !ok {
			panic("controller is not ControllerInterface")
		}
		// call the controller init function
		execController.Init(ctx, GConfig.Listen)

		// call prepare function
		execController.BeforeAction()

		execController.URLMapping()

		method := vc.MethodByName(err.method)
		method.Call([]reflect.Value{})

		// render template
		if err := execController.Render(); err != nil {
			panic(err)
		}

		// finish all runrouter. release resource
		execController.AfterAction()
	}
}


// ErrorHandler registers http.HandlerFunc to each http err code string.
// usage:
// 	gon.ErrorHandler("404",NotFound)
//	gon.ErrorHandler("500",InternalServerError)
func ErrorHandler(code string, h http.HandlerFunc) {
	ErrorMaps[code] = &errorInfo{
		errorType: errorTypeHandler,
		handler:   h,
		method:    code,
	}
}

// render default application error page with error and stack string.
func showErr(err interface{}, ctx *context.Context, stack string) {
	t, _ := template.New("gonerrortemp").Parse(errorTpl)
	data := map[string]string{
		"AppError":      fmt.Sprintf("%s:%v", GConfig.AppName, err),
		"RequestMethod": ctx.Input.Method(),
		"RequestURL":    ctx.Input.URI(),
		"RemoteAddr":    ctx.Input.IP(),
		"Stack":         stack,
		"GonVersion":  VERSION + "[" + VERSION_NAME + "]",
		"GoVersion":     runtime.Version(),
	}
	t.Execute(ctx.ResponseWriter, data)
}