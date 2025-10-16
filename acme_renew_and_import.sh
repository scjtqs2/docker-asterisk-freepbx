#!/bin/bash
# 注意: 该脚本将在 Cron 环境中运行，环境变量可能不会自动加载。
# 该脚本依赖于 /etc/freepbx.env 文件来加载环境变量 (由 run-httpd.sh 创建)。

# --- 设置一个完整的 PATH 环境变量，以确保所有命令都能被找到 ---
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# 从环境文件中加载变量 (如 FQDN)
if [ -f /etc/freepbx.env ]; then
    source /etc/freepbx.env
else
    echo "$(date) - 严重错误: 环境文件 /etc/freepbx.env 未找到！" >> /data/log/acme_renew.log
    exit 1
fi

ACME_HOME="/data/acme"
ACME_SH="$ACME_HOME/acme.sh"
ACME_DNS_TYPE=${ACME_DNS_TYPE:-dns_cf}

# 检查 FQDN 是否已设置
if [ -z "$FQDN" ]; then
    echo "$(date) - 错误: FQDN 环境变量为空，跳过续订。" >> /data/log/acme_renew.log
    exit 1
fi

# 检查 acme.sh 是否存在
if [ ! -f "$ACME_SH" ]; then
    echo "$(date) - 错误: acme.sh 脚本未找到，无法续订。" >> /data/log/acme_renew.log
    exit 1
fi

echo "$(date) - 正在检查并续订证书 $FQDN ..." >> /data/log/acme_renew.log

# 执行续订（acme.sh 会自动判断是否需要续订）
# 使用 --cron 模式，只有在需要时才会真正执行并输出
"$ACME_SH" --cron --home "$ACME_HOME"

# 检查续订后证书文件是否有更新 (通过检查退出码)
# acme.sh --cron 在续订成功时返回 0，未到期跳过时返回 2
RENEW_CODE=$?
if [ $RENEW_CODE -eq 0 ]; then
    echo "$(date) - 证书已成功续订。正在执行 FreePBX 导入和设置..." >> /data/log/acme_renew.log

    # : 定义所有相关的证书文件路径
    CERT_DIR="$ACME_HOME/${FQDN}_ecc"
    SERVER_CERT_FILE="$CERT_DIR/$FQDN.cer"
    KEY_FILE="$CERT_DIR/$FQDN.key"
    CA_CHAIN_FILE="$CERT_DIR/ca.cer"
    FULL_CHAIN_FILE="$CERT_DIR/fullchain.cer"

    # 检查所有必需文件是否存在
    if [ ! -f "$SERVER_CERT_FILE" ] || [ ! -f "$KEY_FILE" ] || [ ! -f "$CA_CHAIN_FILE" ]; then
        echo "$(date) - 【严重错误】: 证书续订成功，但必需的证书文件未找到！" >> /var/log/acme_renew.log
        exit 1
    fi

    ASTERISK_KEYS_DIR="/etc/asterisk/keys"
    IMPORT_CERT_NAME="acme_auto_import"
    mkdir -p "$ASTERISK_KEYS_DIR"

    # 完全模仿 FreePBX 的文件结构进行复制
    echo "$(date) - 正在复制证书文件到 $ASTERISK_KEYS_DIR..." >> /var/log/acme_renew.log
#    cp "$SERVER_CERT_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"
    cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"
    cp "$KEY_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"
    cp "$CA_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME-ca-bundle.crt"
#    cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME-fullchain.crt"

    # 确保所有文件的权限正确
    chown asterisk:asterisk "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME."*
    # 导入和设置证书到 FreePBX (fwconsole)
    echo "$(date) - 正在导入证书到 FreePBX Certificate Management..." >> /data/log/acme_renew.log
    /usr/sbin/fwconsole certificate --import --force

    CERT_ID=$(/usr/sbin/fwconsole certificate --list | \
        grep -vE '^\+|\ ID\ |^\s*$' | \
        grep "$IMPORT_CERT_NAME" | \
        awk -F'|' '{print $2}' | \
        tr -d '[:space:]' | \
        sed 's/^0*//'
    )

    if [[ "$CERT_ID" =~ ^[0-9]+$ ]]; then
        echo "$(date) - 找到证书 ID: $CERT_ID。设置为默认证书并重载 FreePBX 配置..." >> /data/log/acme_renew.log
        /usr/sbin/fwconsole certificate --default="$CERT_ID"
        /usr/sbin/fwconsole reload
        echo "$(date) - 证书导入和重载完成。" >> /data/log/acme_renew.log
    else
        echo "$(date) - 【严重警告】: 证书导入成功，但无法自动获取证书 ID。提取结果: $CERT_ID。" >> /data/log/acme_renew.log
    fi
elif [ $RENEW_CODE -eq 2 ]; then
    echo "$(date) - 证书未到期，无需续订。" >> /data/log/acme_renew.log
else
    echo "$(date) - 证书续订失败！acme.sh 返回错误码: $RENEW_CODE" >> /data/log/acme_renew.log
fi

exit 0
