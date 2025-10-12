#!/bin/bash
# fail2ban 状态检查脚本

echo "=== Fail2Ban 状态 ==="
fail2ban-client status

echo -e "\n=== Asterisk Jail 状态 ==="
fail2ban-client status asterisk

echo -e "\n=== Asterisk Secure Jail 状态 ==="
fail2ban-client status asterisk-secure

echo -e "\n=== 被禁止的 IP 地址 ==="
fail2ban-client status asterisk | grep -A 100 "Banned IP list:" | tail -n +2
fail2ban-client status asterisk-secure | grep -A 100 "Banned IP list:" | tail -n +2
