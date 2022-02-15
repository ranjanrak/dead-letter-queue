package deadletterqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-redis/redis/v8"
)

// ClientParam represent redis queue inputs from user
type ClientParam struct {
	RedisAddr string
	RedisPasw string
	QueueName string
	Ctx       context.Context
	DeadHTTP  []int
}

// InputMsg represents input message to be added to queue
type InputMsg struct {
	Url       string
	ReqMethod string
	PostParam url.Values
	Headers   http.Header
}

// Client represents interface for redis queue
type Client struct {
	redisCli  *redis.Client
	queueName string
	ctx       context.Context
	deadHTTP  []int
}

// New creates new redis client
func New(userParam ClientParam) *Client {
	// Set default redis address
	if userParam.RedisAddr == "" {
		userParam.RedisAddr = "localhost:6379"
	}
	// Set default queue name
	if userParam.QueueName == "" {
		userParam.QueueName = "queue"
	}
	// Set default context
	if userParam.Ctx == nil {
		userParam.Ctx = context.TODO()
	}
	// Set default deadhttp status codes
	// Dead letter queues will store input params for such HTTPs only to retry/debug later-on
	if userParam.DeadHTTP == nil {
		userParam.DeadHTTP = []int{400, 403, 429, 500, 502, 503, 504}
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     userParam.RedisAddr,
		Password: userParam.RedisPasw,
	})
	return &Client{
		redisCli:  rdb,
		queueName: userParam.QueueName,
		ctx:       userParam.Ctx,
		deadHTTP:  userParam.DeadHTTP,
	}
}

// RawExecute performs the HTTP request based on queue params
func (c *Client) RawExecute(msgParam InputMsg, qName string) {
	var postBody io.Reader
	if msgParam.ReqMethod == "POST" || msgParam.ReqMethod == "PUT" {
		// convert post params map into “URL encoded” form
		paramsEncoded := msgParam.PostParam.Encode()
		postBody = bytes.NewReader([]byte(paramsEncoded))
	}
	req, _ := http.NewRequest(msgParam.ReqMethod, msgParam.Url, postBody)

	if msgParam.Headers != nil {
		req.Header = msgParam.Headers
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	c.HandleDeadQueue(res, msgParam, qName)

	body, _ := ioutil.ReadAll(res.Body)
	// Add response body data to redis to be fetched by another worker
	// To:do- Have better key hash to store successful response
	successKey := msgParam.Url + msgParam.ReqMethod
	err = c.redisCli.Set(c.ctx, successKey, string(body), 0).Err()
	if err != nil {
		panic(err)
	}
}

// ExecuteQueueName is wrapper for RawExecute on qName
func (c *Client) ExecuteQueueName(qName string) {
	// fetch all messages available in queue
	queueData := c.GetQueue(qName)
	for _, queue := range queueData {
		c.RawExecute(queue, qName)
	}
}

// ExecuteQueue executes all available messages in normal queue
func (c *Client) ExecuteQueue() {
	// execute only normal queue messages
	c.ExecuteQueueName(c.queueName)
}

// AddMessage adds incoming new message to redis queue
func (c *Client) AddMessage(message InputMsg) {
	// create/update queue
	queueMsg := append(c.GetQueue(c.queueName), message)
	jsonMessage, err := json.Marshal(queueMsg)
	if err != nil {
		panic(err)
	}
	// Add new message to Queue
	err = c.redisCli.Set(c.ctx, c.queueName, jsonMessage, 0).Err()
	if err != nil {
		panic(err)
	}
}

// GetQueue fetches all messages in queue
func (c *Client) GetQueue(qname string) []InputMsg {
	var queueSlice []InputMsg
	val, err := c.redisCli.Get(c.ctx, qname).Result()
	if err != nil {
		// handle empty redis GET
		if val != "" {
			panic(val)
		}
	}
	err = json.Unmarshal([]byte(val), &queueSlice)
	return queueSlice
}

// HandleDeadQueue creates/update dead queue to retry later
func (c *Client) HandleDeadQueue(res *http.Response, msgParam InputMsg, qName string) {
	// Create/add dead letter queue based on user input for deadHTTP
	if Find(c.deadHTTP, res.StatusCode) {
		// Add failed messages to dead letter queue
		qkey := strconv.Itoa(res.StatusCode)
		deadMsg := append(c.GetQueue(qkey), msgParam)
		jsonMessage, err := json.Marshal(deadMsg)
		if err != nil {
			panic(err)
		}
		err = c.redisCli.Set(c.ctx, qkey, jsonMessage, 0).Err()
		if err != nil {
			panic(err)
		}
	}
	// Delete executed message from queue
	newQueue := RemoveIndex(c.GetQueue(qName), 0)
	jsonMessage, err := json.Marshal(newQueue)
	if err != nil {
		panic(err)
	}
	// Reset queue with updated pending messages in queue
	err = c.redisCli.Set(c.ctx, qName, jsonMessage, 0).Err()
	if err != nil {
		panic(err)
	}
}

// ExecuteDeadQueue executes all available messages in the dead queues
func (c *Client) ExecuteDeadQueue() {
	// execute only dead letter queue messages
	for _, deadQue := range c.deadHTTP {
		c.ExecuteQueueName(strconv.Itoa(deadQue))
	}
}

// RemoveIndex removes top most item from Queue
func RemoveIndex(s []InputMsg, index int) []InputMsg {
	return append(s[:index], s[index+1:]...)
}

// Find takes a slice and looks for an element in it. If found it will
// return bool true else false
func Find(httpSlice []int, http int) bool {
	for _, item := range httpSlice {
		if item == http {
			return true
		}
	}
	return false
}
