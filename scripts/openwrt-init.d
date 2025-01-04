#!/bin/sh /etc/rc.common

START=99
USE_PROCD=1
PROG=/usr/bin/go-dns-proxy

get_config() {
    config_get_bool enabled $1 enabled 1
    config_get port $1 port 53
    config_get china_server $1 china_server "120.53.53.53"
    config_get oversea_server $1 oversea_server "1.1.1.1"
    config_get admin_port $1 admin_port 8080
    config_get data_dir $1 data_dir "/etc/go-dns-proxy/data"
    config_get log_level $1 log_level "info"
}

start_service() {
    config_load go-dns-proxy
    config_foreach get_config go-dns-proxy

    [ "$enabled" -eq 0 ] && return

    mkdir -p "$data_dir"

    procd_open_instance
    procd_set_param command $PROG \
        --logLevel "$log_level" \
        start \
        --port "$port" \
        --chinaServer "$china_server" \
        --overSeaServer "$oversea_server" \
        --adminPort "$admin_port" \
        --dataDir "$data_dir"
    
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}

service_triggers() {
    procd_add_reload_trigger "go-dns-proxy"
}

reload_service() {
    stop
    start
} 