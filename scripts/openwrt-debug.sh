#!/bin/sh

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 测试 DNS 服务器
test_dns_server() {
    local server="$1"
    local domain="$2"
    local server_type="$3"
    local timeout=3

    echo -e "\n${YELLOW}测试 $server_type DNS 服务器: $server${NC}"
    echo -e "测试域名: $domain"

    case "$server" in
        https://*)
            # DOH 服务器测试
            echo -e "\n${GREEN}使用 DOH 协议测试...${NC}"
            result=$(curl -s -w "\n%{http_code}" --connect-timeout "$timeout" -H 'accept: application/dns-message' -H 'content-type: application/dns-message' "$server" -o /dev/null)
            http_code=$result

            if [ "$http_code" = "200" ]; then
                echo -e "${GREEN}DOH 服务器连接正常${NC}"
                return 0
            else
                echo -e "${RED}DOH 服务器连接失败 (HTTP 状态码: $http_code)${NC}"
                echo -e "可能的原因:"
                echo -e "1. 服务器地址错误"
                echo -e "2. 网络连接问题"
                echo -e "3. 服务器不支持 DOH 协议"
                return 1
            fi
            ;;
        tls://*)
            # DOT 服务器测试
            server=${server#tls://}
            echo -e "\n${GREEN}使用 DOT 协议测试...${NC}"
            if echo "Q" | timeout "$timeout" openssl s_client -connect "$server:853" >/dev/null 2>&1; then
                echo -e "${GREEN}DOT 服务器连接正常${NC}"
                return 0
            else
                echo -e "${RED}DOT 服务器连接失败${NC}"
                echo -e "可能的原因:"
                echo -e "1. 服务器地址错误"
                echo -e "2. 网络连接问题"
                echo -e "3. 服务器不支持 DOT 协议"
                echo -e "4. 853 端口被封锁"
                return 1
            fi
            ;;
        *)
            # 普通 DNS 服务器测试
            echo -e "\n${GREEN}使用普通 DNS 协议测试...${NC}"
            if ! echo "$server" | grep -q ":"; then
                server="$server:53"
            fi
            
            if nslookup "$domain" "$server" >/dev/null 2>&1; then
                echo -e "${GREEN}DNS 服务器工作正常${NC}"
                return 0
            else
                echo -e "${RED}DNS 查询失败${NC}"
                echo -e "可能的原因:"
                echo -e "1. 服务器地址错误"
                echo -e "2. 网络连接问题"
                echo -e "3. 53 端口被封锁"
                return 1
            fi
            ;;
    esac
}

# 主函数
main() {
    echo -e "${GREEN}开始诊断 DNS 服务器...${NC}"

    # 获取配置
    config_file="/etc/config/go-dns-proxy"
    if [ ! -f "$config_file" ]; then
        echo -e "${RED}错误: 配置文件不存在 ($config_file)${NC}"
        exit 1
    }

    # 读取配置
    china_server=$(uci -q get go-dns-proxy.main.china_server)
    oversea_server=$(uci -q get go-dns-proxy.main.oversea_server)

    if [ -z "$china_server" ]; then
        china_server="120.53.53.53"
        echo -e "${YELLOW}警告: 未配置国内 DNS 服务器，使用默认值 $china_server${NC}"
    fi

    if [ -z "$oversea_server" ]; then
        oversea_server="1.1.1.1"
        echo -e "${YELLOW}警告: 未配置海外 DNS 服务器，使用默认值 $oversea_server${NC}"
    fi

    # 测试国内 DNS
    test_dns_server "$china_server" "www.baidu.com" "国内"
    china_result=$?

    # 测试海外 DNS
    test_dns_server "$oversea_server" "www.google.com" "海外"
    oversea_result=$?

    echo -e "\n${GREEN}诊断结果:${NC}"
    if [ $china_result -eq 0 ]; then
        echo -e "国内 DNS 服务器: ${GREEN}正常${NC}"
    else
        echo -e "国内 DNS 服务器: ${RED}异常${NC}"
    fi

    if [ $oversea_result -eq 0 ]; then
        echo -e "海外 DNS 服务器: ${GREEN}正常${NC}"
    else
        echo -e "海外 DNS 服务器: ${RED}异常${NC}"
    fi

    if [ $china_result -eq 0 ] && [ $oversea_result -eq 0 ]; then
        echo -e "\n${GREEN}所有 DNS 服务器工作正常！${NC}"
        exit 0
    else
        echo -e "\n${RED}存在异常的 DNS 服务器，请检查配置。${NC}"
        exit 1
    fi
}

# 运行主函数
main 