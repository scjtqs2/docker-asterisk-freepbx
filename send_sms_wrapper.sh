#!/bin/bash
# File: send_sms_wrapper.sh
# 避免发送短信内容中包含换行符之类的特殊符号存在，导致短信被截断。
# Read arguments passed from the dialplan
DEVICE=$1
RECIPIENT=$2

# Read the multi-line message from standard input (stdin)
MESSAGE=$(cat)

# Execute the asterisk command. The shell correctly preserves the newlines
# in the "$MESSAGE" variable when quoted like this.
asterisk -rx "quectel sms \"$DEVICE\" \"$RECIPIENT\" \"$MESSAGE\""
