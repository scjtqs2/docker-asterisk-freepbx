#!/bin/bash
# 注意: 该脚本将在 Cron 环境中运行，环境变量可能不会自动加载，需要手动设置。

ACME_HOME="/data/acme"
ACME_SH="$ACME_HOME/acme.sh"
FQDN=${FQDN:-} # !!! 替换为您的域名，或者通过其他方式获取 !!!
ACME_DNS_TYPE=${ACME_DNS_TYPE:-dns_cf}
# 检查 acme.sh 是否存在
if [ ! -f "$ACME_SH" ]; then
    echo "$(date) - 错误: acme.sh 脚本未找到，无法续订。" >> /var/log/acme_renew.log
    exit 1
fi

# $FQDN 为空则不执行

# 确保加载 acme.sh 配置，包括 DNS API 凭证
# 证书凭证（CF_Token, CF_Email等）通常存储在 acme.sh 自身的配置中
# 但为了安全和可靠性，在容器启动时将这些变量导入到 acme.sh 配置中是更佳做法。
# 这里我们假设启动脚本已经成功导入过一次并存储了凭证。

echo "$(date) - 正在检查并续订证书 $FQDN ..." >> /var/log/acme_renew.log

# 执行续订（acme.sh 会自动判断是否需要续订）
"$ACME_SH" --cron --home "$ACME_HOME" --issue -d "$FQDN" --dns ACME_DNS_TYPE --keylength ec-256

if [ $? -eq 0 ]; then
    echo "$(date) - 证书续订检查完成，正在执行 FreePBX 导入和设置..." >> /var/log/acme_renew.log

    # 证书文件路径 (在 /data/acme 下)
    FULL_CHAIN_FILE="$ACME_HOME/$FQDN"_ecc/fullchain.cer
    KEY_FILE="$ACME_HOME/$FQDN"_ecc/$FQDN.key

    # 【新增】: FreePBX 期望的文件名和目录
    ASTERISK_KEYS_DIR="/etc/asterisk/keys"
    IMPORT_CERT_NAME="acme_auto_import"

    # 确保 keys 目录存在
    mkdir -p "$ASTERISK_KEYS_DIR"

    # 【关键步骤】：将证书文件复制到 FreePBX 期望的导入目录，并重命名
    echo "正在复制证书文件到 $ASTERISK_KEYS_DIR..."
    cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"
    cp "$KEY_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"

    # 确保文件权限正确 (asterisk 用户需要访问)
    chown asterisk:asterisk "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"

    # 4. 导入和设置证书到 FreePBX (fwconsole)
    echo "正在导入证书到 FreePBX Certificate Management..."
    # 【修正】: 直接执行 --import，让 FreePBX 扫描目录
    /usr/sbin/fwconsole certificate --import --force

    CERT_ID=$(fwconsole certificate --list | \
        grep -vE '^\+|\ ID\ |^\s*$' | \
        grep "$IMPORT_CERT_NAME" | \
        awk -F'|' '{print $2}' | \
        tr -d '[:space:]' | \
        sed 's/^0*//'  # <-- 【关键修正】: 移除前导零
    )

    # 确保提取的 ID 是有效的数字
    if [[ "$CERT_ID" =~ ^[0-9]+$ ]]; then
        echo "找到证书 ID: $CERT_ID。设置为默认证书并重载 FreePBX 配置..."

        # 证书 ID 已经确认是可靠的数字，执行设置默认证书
        fwconsole certificate --default="$CERT_ID"
        fwconsole reload
    else
        # 打印提取失败原因，并提供手动指导
        echo "【严重警告】: 证书导入成功，但无法自动获取证书 ID。提取结果: $CERT_ID。"
        echo "请手动在 Web 界面中将证书（名称: $IMPORT_CERT_NAME）设置为默认。"
    fi
else
    echo "$(date) - 证书续订失败！" >> /var/log/acme_renew.log
fi
