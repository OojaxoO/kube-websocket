package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
	"sync/atomic"
)

var config atomic.Value

func main()  {
	//读取配置文件
	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath("/etc/pangu/")
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	e := echo.New()
	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	//e.GET("/terminal/:ip/", terminal)
	e.GET("/log/:cluster/:namespace/:name/", containerLog)
	e.GET("/exec/:cluster/:namespace/:name/", execBash)
	e.GET("/service_pod/:namespace/:name/", servicePod)
	e.GET("/deployment/:cluster/:namespace/", deploymentStatus)
	e.GET("/deployment_top/:cluster/:namespace/", deploymentTop)
	e.GET("/guacamole/log/", terminalLog)
	e.Logger.Fatal(e.Start(viper.GetString("bindAddress")))
}
