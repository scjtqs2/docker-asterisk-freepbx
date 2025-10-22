# 说明
> freepbx 安装和配置起来很麻烦，还各种bug。尤其是想要在arm板子上跑支持很少，于是自己做了个镜像自己用。
> 
> 纯 docker运行，支持arm64(rk3588上在跑)和x86

集成了如下内容：

+ asterisk 20
+ [asterisk-chan-quectel](https://github.com/IchthysMaranatha/asterisk-chan-quectel) 模块
+ freepbx 17
+ php8.2
+ acme.sh 自动生成证书脚本
+ postfix通过smtp发送邮件配置
+ fail2ban
+ 短信转发脚本
+ 4g语音转trunk。具体转发到哪个sip账号，在freepbx里面自行配置即可

# sms_send发送短信api
> 查看你配置的`SMS_SEND_PORT`环境变量，这里就以`1285`为例
> 
> 查看你配置的`FORWARD_SECRET`环境变量，这里就用`YOUR_FORWARD_SECRET`为例
> 
> 已添加了一个简单的前端页面。访问`http://<your_server_ip>:<SMS_SEND_PORT>/` 即可
```shell
curl --location --request POST 'http://<your_server_ip>:1285/api/v1/sms/send' \
--header 'Content-Type: application/json' \
--data '{
    "secret": "YOUR_FORWARD_SECRET",
    "recipient": "目标手机号码",
    "message": "这是您的短信内容。\n可以包含换行符。",
    "device": "quectel0"
}'
```

对接demo可以参考 https://github.com/scjtqs2/bot_app_chat/blob/master/sms_asterisk.go

