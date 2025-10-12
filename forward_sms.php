#!/usr/bin/env php
<?php
// forward_sms.php

if ($argc < 7) {
    file_put_contents('/data/log/forward_sms_php.log', date('c') . " - Missing arguments\n", FILE_APPEND);
    exit(1);
}

list(, $secret, $number, $time, $text, $phone_id, $sms_id, $forward_url) = $argv;

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
        'header'  => "Content-Type: application/json\r\n",
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
