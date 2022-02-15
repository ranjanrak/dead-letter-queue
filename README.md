# dead-letter-queue
Tiny go package for dead letter queue based on redis.

## Installation
```
go get -u github.com/ranjanrak/dead-letter-queue
```

## Usage
```go
package main

import (
	"net/http"
	"net/url"

	deadletterqueue "github.com/ranjanrak/dead-letter-queue"
)
func main() {
    // Create new queue instance
    redisQueue := deadletterqueue.New(deadletterqueue.ClientParam{
        RedisAddr: "",
        RedisPasw: "",
        Ctx:       nil,
        QueueName: "",
        DeadHTTP:  []int{400, 429, 500, 502},
    })
    
    rURL := "https://api.kite.trade/orders/regular"
    params := url.Values{}
    
    params.Add("exchange", "NSE")
    params.Add("tradingsymbol", "SBIN")
    params.Add("transaction_type", "BUY")
    params.Add("quantity", "3")
    
    var headers http.Header
    headers = map[string][]string{}
    
    headers.Add("x-kite-version", "3")
    headers.Add("authorization", "token api_key:access_token")
    headers.Add("content-type", "application/x-www-form-urlencoded")
    
    // Queue message params
    queueMsg := deadletterqueue.InputMsg{
        Url:       rURL,
        ReqMethod: "POST",
        PostParam: params,
        Headers:   headers,
    }
    
    // worker that adds message to redis queue
    redisQueue.AddMessage(queueMsg)
    
    // worker that executes normal queue messages
    redisQueue.ExecuteQueue()

    // worker that executes only dead letter queue messages
    redisQueue.ExecuteDeadQueue()
}
```
## Sample response
`redisQueue.GetQueue("queue")`: List all message available in the queue
```
[{https://api.kite.trade/orders/regular POST map[exchange:[BSE] quantity:[3] tradingsymbol:[ONGC] 
transaction_type:[BUY]] map[Authorization:[token api_key:access_token] 
Content-Type:[application/x-www-form-urlencoded] X-Kite-Version:[3]]} 
{https://api.kite.trade/orders/regular 
POST map[exchange:[NSE] quantity:[5] tradingsymbol:[SBIN] transaction_type:[BUY]] map[Authorization:
[api_key:access_token] Content-Type:[application/x-www-form-urlencoded] X-Kite-Version:[3]]}
{https://api.kite.trade/gtt/triggers POST map[exchange:[NSE] quantity:[10] tradingsymbol:[WIPRO] 
transaction_type:[SELL] trigger_values[702.0]] map[Authorization:[token api_key:access_token] 
Content-Type:[application/x-www-form-urlencoded] X-Kite-Version:[3]]}
```