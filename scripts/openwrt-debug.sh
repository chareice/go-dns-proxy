#!/bin/sh

CONFIG_FILE="/etc/config/go-dns-proxy"
TEST_DOMAIN="www.baidu.com"
TEST_DOMAIN_OVERSEA="www.google.com"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件不存在 ($CONFIG_FILE)"
    exit 1
fi

# 获取配置
get_config() {
    local value
    value=$(uci -q get go-dns-proxy.main.$1)
    echo "$value"
}

# 测试普通 DNS 服务器
test_dns() {
    local server=$1
    local domain=$2
    local timeout=2
    
    echo "测试 DNS 服务器: $server"
    echo "测试域名: $domain"
    
    # 提取服务器地址和端口
    local server_addr=${server%:*}
    local server_port=${server#*:}
    if [ "$server_addr" = "$server_port" ]; then
        server_port=53
    fi
    
    # 使用 nslookup 测试
    result=$(nslookup -timeout=$timeout "$domain" "$server" 2>&1)
    if echo "$result" | grep -q "connection timed out"; then
        echo "❌ 连接超时"
        echo "   可能原因："
        echo "   1. 服务器地址错误"
        echo "   2. 服务器未开放 UDP/$server_port 端口"
        echo "   3. 网络连接问题"
        return 1
    elif echo "$result" | grep -q "server can't find"; then
        echo "❌ 域名解析失败"
        echo "   可能原因："
        echo "   1. 上游 DNS 服务器无法解析该域名"
        echo "   2. 域名不存在"
        return 1
    elif echo "$result" | grep -q "Address:"; then
        echo "✅ 连接正常"
        echo "$result" | grep "Address:" | tail -n1
        return 0
    else
        echo "❌ 未知错误"
        echo "$result"
        return 1
    fi
}

# 测试 DOH 服务器
test_doh() {
    local server=$1
    local domain=$2
    local timeout=5
    
    echo "测试 DOH 服务器: $server"
    echo "测试域名: $domain"
    
    # 使用 curl 测试 DOH 服务器
    result=$(curl -s -w "%{http_code}" -o /dev/null --connect-timeout $timeout "$server")
    if [ "$result" = "200" ] || [ "$result" = "400" ]; then
        echo "✅ HTTPS 连接正常"
        return 0
    else
        echo "❌ HTTPS 连接失败 (HTTP 状态码: $result)"
        if [ -z "$result" ]; then
            echo "   可能原因："
            echo "   1. 网络连接问题"
            echo "   2. DNS 解析失败"
            echo "   3. 服务器无响应"
            echo "   4. HTTPS 证书问题"
        fi
        return 1
    fi
}

echo "=== DNS 服务器连通性测试 ==="
echo ""

# 测试国内服务器
china_server=$(get_config china_server)
if [ -n "$china_server" ]; then
    echo "国内服务器测试："
    if echo "$china_server" | grep -q "^https://"; then
        test_doh "$china_server" "$TEST_DOMAIN"
    else
        test_dns "$china_server" "$TEST_DOMAIN"
    fi
else
    echo "❌ 未配置国内服务器"
fi
echo ""

# 测试海外服务器
oversea_server=$(get_config oversea_server)
if [ -n "$oversea_server" ]; then
    echo "海外服务器测试："
    if echo "$oversea_server" | grep -q "^https://"; then
        test_doh "$oversea_server" "$TEST_DOMAIN_OVERSEA"
    else
        test_dns "$oversea_server" "$TEST_DOMAIN_OVERSEA"
    fi
else
    echo "❌ 未配置海外服务器"
fi 