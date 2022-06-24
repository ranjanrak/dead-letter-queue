# dead-letter-queue

A tiny go package to manage HTTP requests with dead letter management/retry. Based on go-redis.

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
    // Create new HTTP request queue instance
    httpQueue := deadletterqueue.New(deadletterqueue.ClientParam{
		RedisAddr: "",
		RedisPasw: "",
		Ctx:       nil,
		QueueName: "",
		DeadHTTP:  []int{400, 403, 429, 500, 502},
	})
    // Request message
    queueMsg := deadletterqueue.InputMsg{
		Name:      "Place TCS Order",
		Url:       "https://api.kite.trade/orders/regular",
		ReqMethod: "POST",
		PostParam: map[string]interface{}{
			"exchange":         "NSE",
			"tradingsymbol":    "TCS",
			"transaction_type": "BUY",
			"quantity":         1,
			"product":          "CNC",
			"order_type":       "MARKET",
			"validity":         "DAY",
		},
		Headers: map[string]interface{}{
			"x-kite-version": 3,
			"authorization":  "token abcd123:efgh1234",
			"content-type":   "application/x-www-form-urlencoded",
		},
	}

	// worker that adds message to redis queue
	err := httpQueue.AddMessage(queueMsg)
	if err != nil {
		log.Fatalf("Error adding msg in queue : %v", err)
	}

	// worker that executes request queues
	httpQueue.ExecuteQueue()

	// worker that executes only dead letter queues
	httpQueue.ExecuteDeadQueue()
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
