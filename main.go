package main

import (
	"github.com/urfave/cli/v2"
	"go-dns-server/server"
	"log"
	"os"
)

func main() {
	app := &cli.App{
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
						Name:  "chinaDOHServerUrl",
						Usage: "国内DOH服务地址",
						Value: "https://120.53.53.53/dns-query",
					},

					&cli.StringFlag{
						Name:  "overSeaDOHServerUrl",
						Usage: "海外DOH服务地址",
						Value: "https://1.1.1.1/dns-query",
					},
				},
				Name:  "start",
				Usage: "start a proxy dns server",
				Action: func(c *cli.Context) error {
					dnsServer := server.NewDnsServer(&server.NewServerOptions{
						ListenPort:          c.Int("port"),
						ApiKey:              c.String("apiKey"),
						BeianCacheFile:      c.String("beianCache"),
						BeianCacheInterval:  c.Int("cacheInterval"),
						ChinaDOHServerUrl:   c.String("chinaDOHServerUrl"),
						OverSeaDOHServerUrl: c.String("overSeaDOHServerUrl"),
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
