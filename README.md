# dead-letter-queue

[![Run Tests](https://github.com/ranjanrak/dead-letter-queue/actions/workflows/go-test.yml/badge.svg)](https://github.com/ranjanrak/dead-letter-queue/actions/workflows/go-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ranjanrak/dead-letter-queue.svg)](https://pkg.go.dev/github.com/ranjanrak/dead-letter-queue)

A tiny go package to manage HTTP requests with dead letter management/retry. Based on go-redis.

## Installation

```
go get -u github.com/ranjanrak/dead-letter-queue
```

- [Usage](#usage)
- [Request](#request)
  - [Adding message](#adding-message)
  - [Delete message from request queue](#delete-message-from-request-queue)
  - [Delete message from dead letter queue](#delete-message-from-dead-letter-queue)
  - [Clear request queue](#clear-request-queue)
  - [Clear deadletter queue](#clear-deadletter-queue)
- [Execute queue](#executerun-message-queue)
  - [Execute request queue](#execute-request-queue)
  - [Execute deadletter queue](#execute-deadletter-queue)
- [Sample response](#sample-response)

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

	// worker that adds message to http queue
	err := httpQueue.AddMessage(queueMsg)
	if err != nil {
		log.Fatalf("Error adding msg in the request queue : %v", err)
	}

	// worker that executes http request queues
	httpQueue.ExecuteQueue()

	// worker that executes only dead letter http queues
	httpQueue.ExecuteDeadQueue()
}
```

## Request

Request represents an HTTP request with all parameters.

### Adding message

Adding an HTTP message to the HTTP queue with all parameters.

```go
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
err := httpQueue.AddMessage(queueMsg)
if err != nil {
    log.Fatalf("Error adding msg in the request queue : %v", err)
}
```

### Delete message from the request queue

Delete request message available in HTTP queue before it's execution with the input message `Name`.

```go
err := httpQueue.DeleteReqMsg("Place TCS Order")
if err != nil {
    log.Fatalf("Error removing msg from the request msg queue : %v", err)
}
```

### Delete message from the dead letter queue

Delete message by the input message `Name` from the Deadletter queue.

```go
err := httpQueue.DeleteDeadMsg("Place TCS Order")
if err != nil {
    log.Fatalf("Error removing msg from the deadletter queue : %v", err)
}
```

### Clear request queue

Clear complete request message queue.

```go
err := httpQueue.ClearReqQueue()
if err != nil {
    log.Fatalf("Error clearing the request queue : %v", err)
}
```

### Clear deadletter queue

Clear complete deadletter message queue.

```go
err := httpQueue.ClearDeadQueue()
if err != nil {
    log.Fatalf("Error clearing the deadletter queue : %v", err)
}
```

### Execute/run message queue

Execute request queue or dead letter queue(i.e failed HTTP request).

### Execute request queue

Execute HTTP requests in the request queue

```go
httpQueue.ExecuteQueue()
```

### Execute deadletter queue

Execute failed HTTP request message i.e dead letter queue

```go
httpQueue.ExecuteDeadQueue()
```

## Sample response

`httpQueue.GetQueue("ReqQueue")`: Lists all the available messages in the http queue

```

[{"Name":"Place TCS Order","Url":"https://api.kite.trade/orders/regular","ReqMethod":"POST",
"PostParam":{"exchange":"NSE","order_type":"MARKET","product":"CNC","quantity":1,
"tradingsymbol":"TCS","transaction_type":"BUY","validity":"DAY"},
"Headers":{"authorization":"token abcd123:efgh1234","content-type":"application/x-www-form-urlencoded",
"x-kite-version":3}},
{"Name":"Post session token","Url":"https://api.kite.trade/session/token","ReqMethod":"POST","PostParam":
{"api_key":"api_key","checksum":"checksum","request_token":"request_token"},"Headers":{"x-kite-version":3}},
..]

```
