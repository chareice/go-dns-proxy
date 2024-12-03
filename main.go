package main

import (
	"fmt"
	"go-dns-proxy/admin"
	"go-dns-proxy/server"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
						Name:  "chinaServer",
						Usage: "国内 DNS 服务地址（支持普通 DNS、DOH 和 DOT）",
						Value: "120.53.53.53",
					},
					&cli.StringFlag{
						Name:  "overSeaServer",
						Usage: "海外 DNS 服务地址（支持普通 DNS、DOH 和 DOT）",
						Value: "1.1.1.1",
					},
					&cli.IntFlag{
						Name:  "adminPort",
						Usage: "管理后台端口",
						Value: 8080,
					},
					&cli.StringFlag{
						Name:  "dataDir",
						Usage: "数据目录路径",
						Value: "./data",
					},
				},
				Name:  "start",
				Usage: "start a proxy dns server",
				Action: func(c *cli.Context) error {
					log.Info("启动DNS代理服务器...")

					// 创建数据目录
					dataDir := c.String("dataDir")
					if err := os.MkdirAll(dataDir, 0755); err != nil {
						return err
					}

					// 初始化 DNS 服务器
					dnsServer, err := server.NewDnsServer(&server.NewServerOptions{
						ListenPort:      c.Int("port"),
						ApiKey:          c.String("apiKey"),
						ChinaServerAddr: c.String("chinaServer"),
						OverSeaServerAddr: c.String("overSeaServer"),
						DBPath:          filepath.Join(dataDir, "dns.db"),
					})
					if err != nil {
						return err
					}

					// 添加数据库日志钩子
					log.AddHook(admin.NewDBHook(dnsServer.GetDB()))

					// 启动管理后台
					adminServer := admin.NewServer(dnsServer.GetDB())
					admin.SetAdminServer(adminServer)
					go func() {
						if err := adminServer.Start(fmt.Sprintf(":%d", c.Int("adminPort"))); err != nil {
							log.WithError(err).Error("管理后台启动失败")
						}
					}()

					log.WithFields(log.Fields{
						"国内DNS":  c.String("chinaServer"),
						"海外DNS":  c.String("overSeaServer"),
						"监听端口":   c.Int("port"),
						"日志级别":   c.String("logLevel"),
						"管理后台端口": c.Int("adminPort"),
					}).Info("服务器配置")

					// 设置信号处理
					sigChan := make(chan os.Signal, 1)
					signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

					// 启动 DNS 服务器
					go dnsServer.Start()

					// 等待信号
					sig := <-sigChan
					log.WithField("signal", sig).Info("收到退出信号")

					// 优雅关闭
					if err := dnsServer.Close(); err != nil {
						log.WithError(err).Error("关闭服务器失败")
					}

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
