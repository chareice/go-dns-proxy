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
    local beian_cache_file
    local beian_cache_interval

    config_load 'go-dns-proxy'
    config_get enabled main 'enabled' '1'
    config_get port main 'port' '53'
    config_get china_server main 'china_server' '114.114.114.114'
    config_get oversea_server main 'oversea_server' '1.1.1.1'
    config_get beian_api_key main 'beian_api_key' ''
    config_get beian_cache_file main 'beian_cache_file' '/etc/go-dns-proxy/beian_cache.json'
    config_get beian_cache_interval main 'beian_cache_interval' '10'

    [ "$enabled" = "1" ] || return

    # 确保缓存目录存在
    mkdir -p "$(dirname "$beian_cache_file")"

    procd_open_instance
    procd_set_param command $PROG start \
        --port "$port" \
        --chinaServer "$china_server" \
        --overSeaServer "$oversea_server" \
        --apiKey "$beian_api_key" \
        --beianCache "$beian_cache_file" \
        --cacheInterval "$beian_cache_interval"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
} 