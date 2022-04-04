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
						Usage:    "dns server listen port",
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
				},
				Name:  "china",
				Usage: "start a china dns server",
				Action: func(c *cli.Context) error {
					port := c.Int("port")
					apiKey := c.String("apiKey")
					beianCache := c.String("beianCache")
					dnsServer := server.NewDnsServer(port, apiKey, beianCache)
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
