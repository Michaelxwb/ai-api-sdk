## 1、elysiver.h-e.top平台
URL：https://elysiver.h-e.top/pg/chat/completions

```yaml
POST /pg/chat/completions HTTP/1.1
Host: elysiver.h-e.top
Connection: keep-alive
Content-Length: 181
New-Api-User: 10121
sec-ch-ua-platform: "macOS"
sec-ch-ua-full-version-list: "Google Chrome";v="143.0.7499.193", "Chromium";v="143.0.7499.193", "Not A(Brand";v="24.0.0.0"
sec-ch-ua: "Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"
sec-ch-ua-bitness: "64"
sec-ch-ua-model: ""
sec-ch-ua-mobile: ?0
sec-ch-ua-arch: "arm"
sec-ch-ua-full-version: "143.0.7499.193"
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36
Content-Type: application/json
sec-ch-ua-platform-version: "15.7.3"
Accept: */*
Origin: https://elysiver.h-e.top
Sec-Fetch-Site: same-origin
Sec-Fetch-Mode: cors
Sec-Fetch-Dest: empty
Referer: https://elysiver.h-e.top/console/playground
Accept-Encoding: gzip, deflate, br, zstd
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Cookie: _clck=1yzm5pq%5E2%5Eg1h%5E0%5E2160; session=MTc2NzUzNjg4MnxEWDhFQVFMX2dBQUJFQUVRQUFEX3hmLUFBQVlHYzNSeWFXNW5EQVFBQW1sa0EybHVkQVFFQVA1UEVnWnpkSEpwYm1jTUNnQUlkWE5sY201aGJXVUdjM1J5YVc1bkRBOEFEV3hwYm5WNFpHOWZNVEF4TWpFR2MzUnlhVzVuREFZQUJISnZiR1VEYVc1MEJBSUFBZ1p6ZEhKcGJtY01DQUFHYzNSaGRIVnpBMmx1ZEFRQ0FBSUdjM1J5YVc1bkRBY0FCV2R5YjNWd0JuTjBjbWx1Wnd3SkFBZGtaV1poZFd4MEJuTjBjbWx1Wnd3TkFBdHZZWFYwYUY5emRHRjBaUVp6ZEhKcGJtY01EZ0FNWVdSek1tVndOa3c0T1VWTXwEDzJ35vBQa7bM2cQIc-S-V2nNR9nMFF6qIipBey3UIQ==; cf_clearance=PwhNnp9Pq1Hww_x9U8rSuNGrh8CoYnYO6pSyxOSo2bo-1769503336-1.2.1.1-yZ2PklpAqJCO_flLa7OUvytMWAmXN9kl92fpQZg6Z6rySaqMtsnWQr.vewsyNeh138uCVMcs0t5SJg77X08EKZ_q54a396ppt5OiPOiyMculZqjraS3GSiB7z3Jn2k3dGg3HIUJ2_nEz9XtBperP8UWHA1NBTA52geTpptbheUnfnWWcS_OxtzmFkqB7_rWHueMSVSGdQKr6CQfQrQOZNro0bwM0WGWwPt2iawYtIpM

{"model":"claude-4.5-sonnet","group":"default","messages":[{"role":"user","content":"测试"}],"stream":false,"temperature":0.7,"top_p":1,"frequency_penalty":0,"presence_penalty":0}
```


```yaml
HTTP/1.1 200 OK
Date: Tue, 27 Jan 2026 08:48:02 GMT
Content-Type: application/json
Transfer-Encoding: chunked
Connection: keep-alive
CF-RAY: 9c46fcf74cb1f5b9-AMS
Server: cloudflare
Content-Encoding: zstd
alt-svc: h3=":443"; ma=86400
referrer-policy: no-referrer-when-downgrade
x-content-type-options: nosniff
x-dns-prefetch-control: off
x-frame-options: SAMEORIGIN
x-oneapi-request-id: 20260127164756439776527Z1chxraP
cf-cache-status: DYNAMIC
Report-To: {"endpoints":[{"url":"https:\/\/a.nel.cloudflare.com\/report\/v4?s=KbjiSOX9sRT1DXqsyox28T0%2Fx4tIve4nSVpB6P5UYGYg0nM3eFAWAiKyxkIXSRE41hPb9w4YVBjD1vCq9a%2BdWcfYrMjmpnimeEo4jdniyarULtXv%2FNG0kmEV%2FK2%2Bp9Ar8Otb"}],"group":"cf-nel","max_age":604800}
Report-To: {"endpoints":[{"url":"https:\/\/csp-reporting.cloudflare.com\/cdn-cgi\/script_monitor\/report?m=0M01Z_1.k1X0RQkwtLbnHKQR4fgDAgdYSRUecSqGVqI-1769503682-1.0.1.1-y.B3lOi3d_Qfhp_NmmipB.Fj2N1QyQeYdU6i6RtNmW8pD3vaj.EfjITaN_wO4AQi9oDhevbYsDgAnyuuDLtMi5X.HtZ1STHgeV8.R.yT13hxQcp2MG.miMvTSeQtnAK9qVE8t3T.l4KuxQoKPkyQa7VqiRcvrzQrLV1SGi5A219HfoEk5T72QlenurVDFJ4c"}],"group":"cf-csp-endpoint","max_age":86400}
NEL: {"success_fraction":0,"report_to":"cf-nel","max_age":604800}
Content-Security-Policy-Report-Only: script-src 'none'; report-uri https://csp-reporting.cloudflare.com/cdn-cgi/script_monitor/report?m=0M01Z_1.k1X0RQkwtLbnHKQR4fgDAgdYSRUecSqGVqI-1769503682-1.0.1.1-y.B3lOi3d_Qfhp_NmmipB.Fj2N1QyQeYdU6i6RtNmW8pD3vaj.EfjITaN_wO4AQi9oDhevbYsDgAnyuuDLtMi5X.HtZ1STHgeV8.R.yT13hxQcp2MG.miMvTSeQtnAK9qVE8t3T.l4KuxQoKPkyQa7VqiRcvrzQrLV1SGi5A219HfoEk5T72QlenurVDFJ4c; report-to cf-csp-endpoint
server-timing: cfL4;desc="?proto=TCP&rtt=204044&min_rtt=203918&rtt_var=76722&sent=5&recv=7&lost=0&retrans=0&sent_bytes=2810&recv_bytes=2565&delivery_rate=14092&cwnd=113&unsent_bytes=0&cid=3c1b6e729f0d13e5&ts=6221&x=0"

{"id":"chatcmpl-9bef329f91cb495a829c17211ef8c","object":"chat.completion","created":1769503681,"model":"claude-4.5-sonnet","choices":[{"index":0,"message":{"role":"assistant","content":"好的，我明白了。您想测试我在不读取当前站点知识的情况下进行回答。\n\n我现在可以直接回答您的问题，而不会调用任何工具来读取文档或学习内容。请随意提出您的问题，我会基于我的通用知识来回答您，而不会访问这个学习平台的特定文档。\n\n您想问什么问题呢？","tool_calls":[]},"finish_reason":"stop"}],"usage":{"prompt_tokens":1078,"completion_tokens":124,"total_tokens":1202}}
```

## 2、deepseek平台
URL：https://api.deepseek.com/chat/completions

```yaml
POST /chat/completions HTTP/1.1
Host: api.deepseek.com
Authorization: Bearer sk-356182ed66f040a2acf415386b250373
Content-Type: application/json
Accept: */*
Connection: close
Content-Length: 181

{
  "model": "deepseek-chat",
  "messages": [
    {
      "role": "user",
      "content": "hello"
    }
  ],
  "temperature": 0.7,
  "stream": true,
  "max_tokens": 1000
}
```


```yaml
HTTP/1.1 200 OK
Date: Thu, 08 Jan 2026 10:16:47 GMT
Content-Type: text/event-stream; charset=utf-8
Connection: close
Set-Cookie: HWWAFSESTIME=1767867405859; path=/
Set-Cookie: HWWAFSESID=2779188a756965b4b2; path=/
cache-control: no-cache
vary: origin, access-control-request-method, access-control-request-headers
access-control-allow-credentials: true
x-ds-trace-id: 51285181f25ec88b8c44027d3d293e68
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Content-Type-Options: nosniff
Server: CW
Content-Length: 9938

data: {"id":"873123fe-56ff-4f1e-a501-13efd06fe283","object":"chat.completion.chunk","created":1767867407,"model":"deepseek-chat","system_fingerprint":"fp_eaab8d114b_prod0820_fp8_kvcache","choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}]}
```

