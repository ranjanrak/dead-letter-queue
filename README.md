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
  - [Delete message from the request queue](#delete-message-from-the-request-queue)
  - [Delete message from the dead letter queue](#delete-message-from-the-dead-letter-queue)
  - [Clear request queue](#clear-request-queue)
  - [Clear deadletter queue](#clear-deadletter-queue)
- [Execute queue](#executerun-message-queue)
  - [Execute request queue](#execute-request-queue)
  - [Execute deadletter queue](#execute-deadletter-queue)
- [Fetch message response status](#fetch-message-response-status)
- [Sample response](#sample-response)

## Usage

```go
package main

import (
    "log"
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

    // Add post params
    postParam := url.Values{}
    postParam.Add("exchange", "NSE")
    postParam.Add("tradingsymbol", "TCS")
    postParam.Add("transaction_type", "BUY")
    postParam.Add("quantity", "1")
    postParam.Add("product", "CNC")
    postParam.Add("order_type", "MARKET")
    postParam.Add("validity", "DAY")

    // Add request header
    var headers http.Header = map[string][]string{}
    headers.Add("x-kite-version", "3")
    headers.Add("authorization", "token api_key:access_token")
    headers.Add("content-type", "application/x-www-form-urlencoded")

    // Request message
    reqMsgOrd := deadletterqueue.InputMsg{
        Name:      "Place TCS Order",
        Url:       "https://api.kite.trade/orders/regular",
        ReqMethod: "POST",
        PostParam: postParam,
        Headers:   headers,
    }

    // worker that adds message to redis queue
    err := httpQueue.AddMessage(reqMsgOrd)
    if err != nil {
        log.Fatalf("Error adding msg in the request queue : %v", err)
    }

    // worker that executes http request queues
    httpQueue.ExecuteQueue()

    // worker that executes dead letter http queues
    httpQueue.ExecuteDeadQueue()
}
```

## Request

Request represents an HTTP request with all parameters.

### Adding message

Adding an HTTP message to the request queue with all parameters.

```go
// Add request header
var headers http.Header = map[string][]string{}
headers.Add("x-kite-version", "3")
headers.Add("authorization", "token api_key:access_token")

// Request message
queueMsg := deadletterqueue.InputMsg{
    Name:      "Fetch order book",
    Url:       "https://api.kite.trade/orders",
    ReqMethod: "GET",
    PostParam: nil,
    Headers:   headers,
}
err := httpQueue.AddMessage(queueMsg)
if err != nil {
    log.Fatalf("Error adding msg in the request queue : %v", err)
}
```

### Delete message from the request queue

Delete request message available in the queue before it's execution with the input message `Name`.

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

## Execute/run message queue

Execute request queue or dead letter queue(i.e failed HTTP request).

### Execute request queue

Execute HTTP requests in the request queue

```go
httpQueue.ExecuteQueue()
```

### Execute deadletter queue

Execute failed HTTP request message i.e dead letter queue.

```go
httpQueue.ExecuteDeadQueue()
```

## Fetch message response status

Fetch response body of an given message name, post it's execution.

```go
status, err := httpQueue.MessageStatus("Place TCS Order")
if err != nil {
    log.Fatalf("Error %v", err)
}
log.Printf("Response status %v: ", status)

```

Sample responses

```
Response status : {"status":"success","data":{"order_id":"220627001805439"}}

Response status : {"status":"error",
"message":"Your order price is lower than the current [lower circuit limit]",
"data":null,"error_type":"InputException"}

```

## Sample response

`httpQueue.GetQueue("ReqQueue")`: Lists all the available messages in the http queue

```

[{"Name":"Place TCS Order","Url":"https://api.kite.trade/orders/regular",
"ReqMethod":"POST","PostParam":{"exchange":"NSE","order_type":"MARKET",
"product":"CNC","quantity":1,"tradingsymbol":"TCS","transaction_type":"BUY",
"validity":"DAY"},"Headers":{"authorization":"token abcd123:efgh1234",
"content-type":"application/x-www-form-urlencoded","x-kite-version":3}},
{"Name":"Post session token","Url":"https://api.kite.trade/session/token",
"ReqMethod":"POST","PostParam":{"api_key":"api_key","checksum":"checksum",
"request_token":"request_token"},"Headers":{"x-kite-version":3}},
..]

```
