#!/bin/sh /etc/rc.common

START=99
USE_PROCD=1
PROG=/usr/bin/go-dns-proxy

start_service() {
    local enabled
    local port
    local china_server
    local oversea_server
    local beian_api_key
    local admin_port
    local data_dir
    local log_level

    config_load 'go-dns-proxy'
    config_get enabled main 'enabled' '1'
    config_get port main 'port' '53'
    config_get china_server main 'china_server' '114.114.114.114'
    config_get oversea_server main 'oversea_server' '1.1.1.1'
    config_get beian_api_key main 'beian_api_key' ''
    config_get admin_port main 'admin_port' '8080'
    config_get data_dir main 'data_dir' '/etc/go-dns-proxy/data'
    config_get log_level main 'log_level' 'info'

    [ "$enabled" = "1" ] || return

    # 确保数据目录存在
    mkdir -p "$data_dir"

    procd_open_instance
    procd_set_param command $PROG \
        --logLevel "$log_level" \
        start \
        --port "$port" \
        --chinaServer "$china_server" \
        --overSeaServer "$oversea_server" \
        --apiKey "$beian_api_key" \
        --adminPort "$admin_port" \
        --dataDir "$data_dir"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
} 