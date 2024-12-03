package main

import (
	"go-dns-proxy/server"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func init() {
	// 设置日志格式
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetOutput(os.Stdout)
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "logLevel",
					Usage: "日志级别 (debug/info/warn/error)",
					Value: "info",
			},
		},
		Before: func(c *cli.Context) error {
			// 设置日志级别
			level, err := log.ParseLevel(c.String("logLevel"))
			if err != nil {
				level = log.InfoLevel
			}
			log.SetLevel(level)
			return nil
		},
		Commands: []*cli.Command{
			{
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "port",
						Usage:    "dns server port",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "apiKey",
						Usage:    "apiKey for query beian domain",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "beianCache",
						Usage:    "备案鉴定的缓存文件地址",
						Required: true,
					},
					&cli.IntFlag{
						Name:  "cacheInterval",
						Usage: "备案缓存写入间隔",
						Value: 10,
					},
					&cli.StringFlag{
						Name:  "chinaServer",
						Usage: "国内 DNS 服务地址（支持普通 DNS、DOH 和 DOT）",
						Value: "120.53.53.53",
					},
					&cli.StringFlag{
						Name:  "overSeaServer",
						Usage: "海外 DNS 服务地址（支持普通 DNS、DOH 和 DOT）",
						Value: "1.1.1.1",
					},
				},
				Name:  "start",
				Usage: "start a proxy dns server",
				Action: func(c *cli.Context) error {
					log.Info("启动DNS代理服务器...")
					log.WithFields(log.Fields{
						"国内DNS":  c.String("chinaServer"),
						"海外DNS":  c.String("overSeaServer"),
						"监听端口":   c.Int("port"),
						"缓存间隔":   c.Int("cacheInterval"),
						"缓存文件":   c.String("beianCache"),
						"日志级别":   c.String("logLevel"),
					}).Info("服务器配置")

					dnsServer := server.NewDnsServer(&server.NewServerOptions{
						ListenPort:         c.Int("port"),
						ApiKey:             c.String("apiKey"),
						BeianCacheFile:     c.String("beianCache"),
						BeianCacheInterval: c.Int("cacheInterval"),
						ChinaServerAddr:    c.String("chinaServer"),
						OverSeaServerAddr:  c.String("overSeaServer"),
					})
					dnsServer.Start()
					return nil
				},
			},
			{
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "port",
						Usage: "dns server listen port",
					},
				},
				Name:  "oversea",
				Usage: "start a oversea server",
				Action: func(c *cli.Context) error {
					log.Info("启动海外DNS服务器...")
					dnsServer := server.NewOverseaDnsServer()
					dnsServer.Start()
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
