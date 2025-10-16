#!/usr/bin/env php
<?php
// forward_sms.php

if ($argc < 7) {
    file_put_contents('/data/log/forward_sms_php.log', date('c') . " - Missing arguments. Expected 6, got " . ($argc-1) . "\n", FILE_APPEND);
    exit(1);
}

// 【修改】: 重新排序参数，$text 不再从这里获取
list(, $secret, $number, $time, $phone_id, $sms_id, $forward_url) = $argv;

// 【新增】: 从标准输入 (stdin) 读取完整的短信内容
// 从标准输入 (stdin) 读取完整的短信内容
$raw_text = stream_get_contents(STDIN);

// 【关键修改】：使用 trim() 函数去掉开头和结尾的空白字符（包括换行符）
$text = trim($raw_text);

// 构建 JSON 数据
$data = [
    'secret' => $secret,
    'number' => $number,
    'time' => $time,
    'text' => $text,
    'source' => 'asterisk',
    'phone_id' => $phone_id,
    'sms_id' => $sms_id,
    'timestamp' => $time,
];

$options = [
    'http' => [
        'header'  => "Content-Type: application/json",
        'method'  => 'POST',
        'content' => json_encode($data, JSON_UNESCAPED_UNICODE),
        'timeout' => 30
    ]
];

$context  = stream_context_create($options);
$result = @file_get_contents($forward_url, false, $context);

$log_line = date('c') . " - Forward SMS result: " . ($result ?: 'failed') . "\n";
file_put_contents('/data/log/sms_forward.log', $log_line, FILE_APPEND);
?>
