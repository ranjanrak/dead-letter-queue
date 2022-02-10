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
    queueMsg := map[string]interface{}{
        "url":       rURL,
        "reqMethod": "POST",
        "postParam": params,
        "headers":   headers,
    }
    
    // worker that adds message to redis queue
    redisQueue.AddMessage(queueMsg)
    
    // worker that executes all available queues
    redisQueue.ExecuteQueue()
}
