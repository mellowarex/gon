package gon

import (
	"fmt"
	"github.com/mellocraft/gon/logs"
	"github.com/mellocraft/gon/utils"
	"html/template"
	"io"
	"net/http"
	"os"
	"runtime"
)

var (
	// error500 hold default 5xx error page
	error500 = "errors/5xx.tpl"
	// error400 hold default 4xx error page
	error400 = "errors/4xx.tpl"
)

var error4xxTpl = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
		<title>{{.Title}}</title>
		<style type="text/css">
			* {
				margin:0;
				padding:0;
			}

			body {
				background-color:#EFEFEF;
				font: .9em "Lucida Sans Unicode", "Lucida Grande", sans-serif;
			}

			#wrapper{
				width:600px;
				margin:40px auto 0;
				text-align:center;
				-moz-box-shadow: 5px 5px 10px rgba(0,0,0,0.3);
				-webkit-box-shadow: 5px 5px 10px rgba(0,0,0,0.3);
				box-shadow: 5px 5px 10px rgba(0,0,0,0.3);
			}

			#wrapper h1{
				color:#FFF;
				text-align:center;
				margin-bottom:20px;
			}

			#wrapper a{
				display:block;
				font-size:.9em;
				padding-top:20px;
				color:#FFF;
				text-decoration:none;
				text-align:center;
			}

			#container {
				width:600px;
				padding-bottom:15px;
				background-color:#FFFFFF;
			}

			.navtop{
				height:40px;
				background-color:#24B2EB;
				padding:13px;
			}

			.content {
				padding:10px 10px 25px;
				background: #FFFFFF;
				margin:;
				color:#333;
			}

			a.button{
				color:white;
				padding:15px 20px;
				text-shadow:1px 1px 0 #00A5FF;
				font-weight:bold;
				text-align:center;
				border:1px solid #24B2EB;
				margin:0px 200px;
				clear:both;
				background-color: #24B2EB;
				border-radius:100px;
				-moz-border-radius:100px;
				-webkit-border-radius:100px;
			}

			a.button:hover{
				text-decoration:none;
				background-color: #24B2EB;
			}

		</style>
	</head>
	<body>
		<div id="wrapper">
			<div id="container">
				<div class="navtop">
					<h1>{{.Title}}</h1>
				</div>
				<div id="content">
					{{.Content}}
					<a href="/" title="Home" class="button">Go Home</a><br />

					<br>Powered by Gon
				</div>
			</div>
		</div>
	</body>
</html>
`
var error5xxTpl = error4xxTpl

func init() {
	// if default error pages exists
	// override default error html
	if utils.FileExists(error500) {
		file, err := os.Open(error500)
		if err != nil {
			logs.Warn(fmt.Sprintf("failed to open file: %s cause: %v; reverting to default error page", error500, err))
		}
		defer file.Close()
		error, err := io.ReadAll(file)
		error5xxTpl = string(error)
		if err != nil {
			logs.Warn(fmt.Sprintf("failed to read file: %s cause: %v", error500, err))
		}
	}
	if utils.FileExists(error400) {
		file, err := os.Open(error400)
		if err != nil {
			logs.Warn(fmt.Sprintf("failed to open file: %s cause: %v; reverting to default error page", error400, err))
		}
		defer file.Close()
		error, err := io.ReadAll(file)
		error4xxTpl = string(error)
		if err != nil {
			logs.Warn(fmt.Sprintf("failed to read file: %s cause: %v", error400, err))
		}
	}
}

// show 401 unauthorized error.
func unauthorized(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		401,
		"<br>The page you have requested can't be authorized."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>The credentials you supplied are incorrect"+
			"<br>There are errors in the website"+
			"</ul>",
	)
}

// show 402 Payment Required
func paymentRequired(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		402,
		"<br>The page you have requested has Payment Required."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>The credentials you supplied are incorrect"+
			"<br>There are errors in the website"+
			"</ul>",
	)
}

// show 403 forbidden error.
func forbidden(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		403,
		"<br>The page you have requested is forbidden."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>Your address may be blocked"+
			"<br>The site may be disabled"+
			"<br>You need to log in"+
			"</ul>",
	)
}

// show 422 missing xsrf token
func missingxsrf(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		422,
		"<br>The page you have requested is forbidden."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>you have connection security is not authenticating"+
			"</ul>",
	)
}

// show 417 invalid xsrf token
func invalidxsrf(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		417,
		"<br>The page you have requested is forbidden."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>required security measures are missing"+
			"</ul>",
	)
}

// show 404 not found error.
func notFound(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		404,
		"<br>The page you have requested is no longer available"+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>The page has moved"+
			"<br>The page no longer exists"+
			"</ul>",
	)
}

// show 405 Method Not Allowed
func methodNotAllowed(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		405,
		"<br>The method you have requested Not Allowed."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br>The method specified is not allowed by the resource identified by the Request-URI"+
			"</ul>",
	)
}

// show 500 internal server error.
func internalServerError(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		500,
		"<br>The page you have requested is down right now."+
			"<br><br><ul>"+
			"<br>Please try again later, this issue has been reported to the website administrator"+
			"<br></ul>",
	)
}

// show 501 Not Implemented.
func notImplemented(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		501,
		"<br>The page you have requested is Not Implemented."+
			"<br><br><ul>"+
			"<br>Please try again later, this issue has been reported to the website administrator"+
			"<br></ul>",
	)
}

// show 502 Bad Gateway.
func badGateway(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		502,
		"<br>The page you have requested is down right now."+
			"<br><br><ul>"+
			"<br>The server, while acting as a gateway or proxy, received an invalid response from the upstream server it accessed in attempting to fulfill the request."+
			"<br>Please try again later, this issue has been reported to the website administrator"+
			"<br></ul>",
	)
}

// show 503 service unavailable error.
func serviceUnavailable(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		503,
		"<br>The page you have requested is unavailable."+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br><br>The page is overloaded"+
			"<br>Please try again later."+
			"</ul>",
	)
}

// show 504 Gateway Timeout.
func gatewayTimeout(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		504,
		"<br>The page you have requested is unavailable"+
			"<br>Perhaps you are here because:"+
			"<br><br><ul>"+
			"<br><br>The server, while acting as a gateway or proxy, did not receive a timely response from the server specified by the URI."+
			"<br>Please try again later."+
			"</ul>",
	)
}

// show 413 Payload Too Large
func payloadTooLarge(rw http.ResponseWriter, r *http.Request) {
	responseError(rw, r,
		413,
		`<br>The page you have requested is unavailable.
		 <br>Perhaps you are here because:<br><br>
		 <ul>
			<br>The request entity is larger than limits defined by server.
			<br>Please change the request entity and try again.
		 </ul>
		`,
	)
}

// M is Map shortcut
type M map[string]interface{}

func responseError(rw http.ResponseWriter, r *http.Request, errCode int, errContent string) {
	errorTpl := errorTpl
	if GConfig.EnvMode == PROD {
		if errCode >= 400 {
			errorTpl = error4xxTpl
		}
		errorTpl = error5xxTpl
	}
	t, _ := template.New("gonerrortemp").Parse(errorTpl)
	data := M{
		"Title":        http.StatusText(errCode),
		"Content":      template.HTML(errContent),

		"AppError":      fmt.Sprintf("Error : %v", errCode),
		"RequestMethod": r.Method,
		"RequestURL":    r.RequestURI,
		"Stack":         "[stack]",

		"GonVersion":  		VERSION ,
		"GonVersionName": "[" + VERSION_NAME + "]",
		"GoVersion":     runtime.Version(),
	}
	t.Execute(rw, data)
}

var errorTpl = `
<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <title>Gon Application Error</title>
    <style>
        html, body, body * {padding: 0; margin: 0;}
        #header {background: #A31515; padding: 20px 10px;}
        #header h2{color: #fff}
        #footer {border-top:solid 1px #aaa; padding: 5px 10px; font-size: 12px; color:green;}
        #content {padding: 5px;}
        #content .stack b{ font-size: 13px; color: red;}
        #content .stack pre{padding-left: 10px;}
        td.t {text-align: right; padding-right: 5px; color: #888;}
    </style>
    <script type="text/javascript">
    </script>
</head>
<body>
    <div id="header">
        <h2>{{.AppError}}</h2>
    </div>
    <div id="content">
        <table>
            <tr>
                <td class="t">Request Method: </td><td>{{.RequestMethod}}</td>
            </tr>
            <tr>
                <td class="t">Request URL: </td><td>{{.RequestURL}}</td>
            </tr>
        </table>
        <div class="stack">
            <b>Stack</b>
            <pre>{{.Stack}}</pre>
        </div>
    </div>
    <div id="footer">
        <p>Gon {{.GonVersion}} {{.GonVersionName}}</p>
        <p>Golang version: {{.GoVersion}}</p>
    </div>
</body>
</html>
`