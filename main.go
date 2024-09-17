package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"multi-app-relay-service/pkg/app"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type DatabricksAppsHeaders struct {
	XForwardedHost  string `json:"X-Forwarded-Host"`
	XForwardedProto string `json:"X-Forwarded-Proto"`
	XForwardedFor   string `json:"X-Forwarded-For"`
	XRealIp         string `json:"X-Real-Ip"`
	XRequestId      string `json:"X-Request-Id"`
}

func (d *DatabricksAppsHeaders) FromHeaders(headers http.Header) {
	d.XForwardedHost = headers.Get("X-Forwarded-Host")
	d.XForwardedProto = headers.Get("X-Forwarded-Proto")
	d.XForwardedFor = headers.Get("X-Forwarded-For")
	d.XRealIp = headers.Get("X-Real-Ip")
	d.XRequestId = headers.Get("X-Request-Id")
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func managementUIProxy(manager *app.Manager) func(c *gin.Context) {
	//Get the app name from the URL

	return func(c *gin.Context) {
		proxyPath := c.Param("proxyPath")
		method := c.Request.Method
		if method == http.MethodGet && (proxyPath == "/_logz" || proxyPath == "/_logs/") {
			logs := manager.ManagementApp.Logs()
			if logs == "" {
				c.String(200, "No logs yet")
				return
			}
			c.String(200, logs)
			return
		}

		rawHost := fmt.Sprintf("0.0.0.0:%d", app.ManagementPort)
		rawUrl := fmt.Sprintf("http://%s", rawHost)
		//Check if the app is running
		remote, err := url.Parse(rawUrl)
		if err != nil {
			panic(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(remote)

		headers := DatabricksAppsHeaders{}
		headers.FromHeaders(c.Request.Header)
		//Define the director func
		//This is a good place to log, for example
		proxy.Director = func(req *http.Request) {
			//fmt.Println("BEFORE: Proxying request to", req.Host, "for app",
			//	manager.ManagementApp.Name, "path",
			//	proxyPath, "method", method, "headers", req.Header)

			req.Header = c.Request.Header
			// Determine the scheme based on TLS presence
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}

			// Check if WebSocket upgrade is requested
			isWebSocket := c.Request.Header.Get("Upgrade") == "websocket"

			// Set appropriate scheme for WebSocket
			if isWebSocket {
				if c.Request.TLS != nil {
					scheme = "wss"
				} else {
					scheme = "ws"
				}
			}

			req.Header.Set("X-Forwarded-Host", coalesce(headers.XForwardedHost, "localhost:8000"))
			req.Header.Set("X-Forwarded-Preferred-Username", c.Request.Header.Get("X-Forwarded-Preferred-Username"))
			req.Header.Set("X-Forwarded-Proto", scheme)

			req.Host = remote.Host
			req.Header.Set("Host", rawHost)
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = c.Param("proxyPath")

			//fmt.Println("AFTER: Proxying request to", req.Host, "for app",
			//	manager.ManagementApp.Name, "path",
			//	proxyPath, "method", method, "headers", headers, "scheme", scheme)
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func makeProxy(manager *app.Manager) func(c *gin.Context) {
	return func(c *gin.Context) {
		//Get the app name from the URL
		appName := c.Param("appName")
		proxyPath := c.Param("proxyPath")
		method := c.Request.Method
		//Get the app from the manager
		thisApp, err := manager.GetApp(appName)
		if err != nil {
			c.JSON(404, gin.H{
				"message": "App not found or not running",
			})
			return
		}
		if method == http.MethodGet && (proxyPath == "/_logz" || proxyPath == "/_logz/") {
			logs := thisApp.Logs()
			if logs == "" {
				c.String(200, "No logs yet")
				return
			}
			c.String(200, logs)
			return
		}
		if thisApp.Status != app.StatusRunning {
			c.JSON(400, gin.H{
				"message": "App is not ready yet. Please try again later. Make sure you started the app",
			})
			return
		}
		appPort, err := manager.GetAppPort(appName)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "App port not found",
			})
			return
		}

		rawUrl := fmt.Sprintf("http://localhost:%d", appPort)
		//Check if the app is running
		remote, err := url.Parse(rawUrl)
		if err != nil {
			panic(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(remote)
		//Define the director func
		//This is a good place to log, for example

		headers := DatabricksAppsHeaders{}
		headers.FromHeaders(c.Request.Header)

		proxy.Director = func(req *http.Request) {
			//fmt.Println("Proxying request to", c.Request.Host, "for app", appName, "path", proxyPath, "method", method)
			req.Header = c.Request.Header
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			req.Header.Set("X-Forwarded-Host", coalesce(headers.XForwardedHost, "localhost:8000"))
			req.Header.Set("X-Forwarded-Preferred-Username", c.Request.Header.Get("X-Forwarded-Preferred-Username"))
			req.Header.Set("X-Forwarded-Proto", scheme)
			req.Header.Set("Host", remote.Host)
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host

			config, err := manager.GetAppConfig(appName)
			if err == nil && config.PassFullProxyPath == false {
				req.URL.Path = c.Param("proxyPath")
			} else {
				req.URL.Path = fmt.Sprintf("/relay/%s%s", appName, proxyPath)
			}
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func main() {

	appManager, err := app.NewManagerFromYaml("multi-app.yaml")
	if err != nil {
		panic(err)
	}

	err = appManager.StageCode() // Clone the repos
	if err != nil {
		panic(err)
	}

	err = appManager.StageUICode()
	if err != nil {
		panic(err)
	}
	err = appManager.RunManager.RunApp(appManager.ManagementApp)
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	//Create a catchall route
	//redirect to management
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/management/")
	})
	r.Any("/management/*proxyPath", managementUIProxy(appManager))
	r.GET("/apps", func(c *gin.Context) {
		appStatuses := make(map[string]string)
		for _, thisApp := range appManager.AllApps {
			appStatuses[thisApp.Name] = thisApp.Status.String()
		}
		appStatuses[appManager.ManagementApp.Name] = appManager.ManagementApp.Status.String()
		c.JSON(200, gin.H{
			"cfg":      appManager.AppsConfig,
			"ports":    appManager.AppPorts,
			"statuses": appStatuses,
		})
	})
	r.Any("/:appName/kill", func(c *gin.Context) {
		appName := c.Param("appName")
		myapp, err := appManager.GetApp(appName)
		if err != nil {
			c.JSON(404, gin.H{
				"message": "App not found",
			})
			return
		}
		err = appManager.RunManager.StopApp(myapp)
		if err != nil {
			c.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "App killed",
		})
	})

	r.Any("/:appName/start", func(c *gin.Context) {
		appName := c.Param("appName")
		myapp, err := appManager.GetApp(appName)
		if err != nil {
			c.JSON(404, gin.H{
				"message": "App not found",
			})
			return
		}
		err = appManager.RunManager.RunApp(myapp)
		if err != nil {
			c.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "App Started",
		})
	})

	r.Any("/relay/:appName/*proxyPath", makeProxy(appManager))
	r.Run(":8000")
}
