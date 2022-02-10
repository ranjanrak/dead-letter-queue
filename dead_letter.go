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

// MsgValue stores each queue message fields
type MsgValue map[string]interface{}

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
func (c *Client) RawExecute(msgParam MsgValue) {
	var postBody io.Reader
	// Fetch all request fields from incoming message
	requestMethod := msgParam["reqMethod"].(string)
	requestPostData := msgParam["postParam"].(url.Values)
	requestURL := msgParam["url"].(string)
	requestHeader := msgParam["headers"].(http.Header)

	if requestMethod == "POST" || requestMethod == "PUT" {
		// convert params map into “URL encoded” form
		paramsEncoded := requestPostData.Encode()
		postBody = bytes.NewReader([]byte(paramsEncoded))
	}
	req, _ := http.NewRequest(requestMethod, requestURL, postBody)

	if requestHeader != nil {
		req.Header = requestHeader
	}

	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()

	c.HandleDeadQueue(res, msgParam)

	body, _ := ioutil.ReadAll(res.Body)
	// Add response body data to redis to be fetched by another worker
	// To:do- Have better key hash to store successful response
	successKey := requestURL + requestMethod
	err := c.redisCli.Set(c.ctx, successKey, string(body), 0).Err()
	if err != nil {
		panic(err)
	}
}

// ExecuteQueue is wrapper for RawExecute
func (c *Client) ExecuteQueue() {
	// fetch all messages available in queue
	queueData := c.GetQueue()

	for _, queue := range queueData {
		c.RawExecute(queue)
	}
}

// AddMessage adds incoming new message to redis queue
func (c *Client) AddMessage(message MsgValue) {
	// create/update queue
	queueMsg := append(c.GetQueue(), message)
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
func (c *Client) GetQueue() []MsgValue {
	var queueSlice []MsgValue
	val, err := c.redisCli.Get(c.ctx, c.queueName).Result()
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
func (c *Client) HandleDeadQueue(res *http.Response, msgParam MsgValue) {
	// Create/add dead letter queue based on user input
	if Find(c.deadHTTP, res.StatusCode) {
		err := c.redisCli.Set(c.ctx, strconv.Itoa(res.StatusCode), msgParam, 0).Err()
		if err != nil {
			panic(err)
		}
	}
	// Delete executed message from queue
	newQueue := RemoveIndex(c.GetQueue(), 0)
	jsonMessage, err := json.Marshal(newQueue)
	if err != nil {
		panic(err)
	}
	// Reset queue with updated pending messages in queue
	err = c.redisCli.Set(c.ctx, c.queueName, jsonMessage, 0).Err()
	if err != nil {
		panic(err)
	}
}

// RemoveIndex removes top most item from Queue
func RemoveIndex(s []MsgValue, index int) []MsgValue {
	return append(s[:index], s[index+1:]...)
}

// Find takes a slice and looks for an element in it. If found it will
// return bool true else false
func Find(slice []int, val int) bool {
	for item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
