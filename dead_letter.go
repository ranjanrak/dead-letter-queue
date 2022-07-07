package deadletterqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
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

// Client represents interface for redis queue
type Client struct {
	redisCli  *redis.Client
	queueName string
	ctx       context.Context
	deadHTTP  []int
}

// InputMsg represents input message to be added to queue
type InputMsg struct {
	Name      string
	Url       string
	ReqMethod string
	PostParam url.Values
	Headers   http.Header
}

// Constants
const (
	// Queue type
	QueueReq  = "request"
	QueueDead = "dead"
)

// New creates new redis client
func New(userParam ClientParam) *Client {
	// Set default redis address
	if userParam.RedisAddr == "" {
		userParam.RedisAddr = "localhost:6379"
	}
	// Set default queue name
	if userParam.QueueName == "" {
		userParam.QueueName = "ReqQueue"
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

// AddMessage adds incoming new HTTP request message to redis queue
func (c *Client) AddMessage(message InputMsg) error {
	// create/update queue
	return c.SetQueue(c.queueName, message)
}

// ExecuteQueue executes all available messages in the normal queue
func (c *Client) ExecuteQueue() {
	// execute only normal queue messages
	c.ExecuteQueueName(c.queueName)
}

// ExecuteDeadQueue executes all available messages in the dead queues
func (c *Client) ExecuteDeadQueue() {
	// execute only dead letter queue messages
	for _, deadQue := range c.deadHTTP {
		c.ExecuteQueueName(strconv.Itoa(deadQue))
	}
}

// ExecuteQueueName is wrapper for RawExecute on qName queue
func (c *Client) ExecuteQueueName(qName string) {
	// fetch all messages available in queue
	msgQueue := c.GetQueue(qName)
	if len(msgQueue) > 0 {
		for _, queue := range msgQueue {
			c.RawExecute(queue, qName)
		}
	} else {
		log.Printf("No messages in %v queue to execute", qName)
	}
}

// RawExecute performs the HTTP request based on request params
func (c *Client) RawExecute(msg InputMsg, qName string) {
	var postBody io.Reader
	if msg.ReqMethod == "POST" || msg.ReqMethod == "PUT" {
		// convert post params map into “URL encoded”
		if msg.PostParam != nil {
			paramsEncoded := msg.PostParam.Encode()
			postBody = bytes.NewReader([]byte(paramsEncoded))
		}
	}
	req, _ := http.NewRequest(msg.ReqMethod, msg.Url, postBody)

	// Add all request headers to the http request
	if msg.Headers != nil {
		req.Header = msg.Headers
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request : %v", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body %v", err)
	}
	// Store response body data
	c.MessageResponse(msg.Name, string(body))

	c.HandleDeadQueue(res, msg, qName)
}

// MessageResponse stores response body of the request body
func (c *Client) MessageResponse(msgName string, response string) {
	err := c.redisCli.Set(c.ctx, msgName, response, 0).Err()
	if err != nil {
		log.Printf("Error updating response for the req message %s", msgName)
	}
}

// HandleDeadQueue creates/update dead queue to retry later
func (c *Client) HandleDeadQueue(res *http.Response, msg InputMsg, qName string) {
	// Create/add dead letter queue based on user input for deadHTTP
	if Find(c.deadHTTP, res.StatusCode) {
		// Alert user with failed status for HTTP request
		log.Printf("Request msg %s, failed with status %s", msg.Name, res.Status)
		// Add failed messages to dead letter queue
		qkey := strconv.Itoa(res.StatusCode)
		err := c.SetQueue(qkey, msg)
		if err != nil {
			log.Fatalf("Error adding dead queue : %v", err)
		}
	}
	// Delete executed message from the redis list
	err := c.redisCli.LTrim(c.ctx, qName, 1, -1).Err()
	if err != nil {
		log.Fatalf("Error removing the queue member: %v", err)
	}
}

// Fetch message response status
func (c *Client) MessageStatus(msgName string) (string, error) {
	val, err := c.redisCli.Get(c.ctx, msgName).Result()
	return val, err
}

// Delete message by message name from request queue
func (c *Client) DeleteReqMsg(msgName string) error {
	return c.DelMsg(c.queueName, msgName)
}

// Delete message by name from Deadletter queue
func (c *Client) DeleteDeadMsg(msgName string) error {
	// Search and delete msg name from all declared dead http queue
	for _, value := range c.deadHTTP {
		err := c.DelMsg(strconv.Itoa(value), msgName)
		if err != nil {
			return err
		}
	}
	return nil
}

// Remove message from the requested queue
func (c *Client) DelMsg(queName string, msgName string) error {
	// Fetch message detail with message name
	msg, err := Marshalmsg(c.MsgDetail(queName, msgName))
	if err != nil {
		return err
	}
	err = c.redisCli.LRem(c.ctx, queName, 0, msg).Err()
	if err != nil {
		return err
	}
	return nil
}

// Clear complete request queue
func (c *Client) ClearReqQueue() error {
	return c.ClearQueue(c.queueName)
}

// Cleat complete dead letter queue
func (c *Client) ClearDeadQueue() error {
	for _, value := range c.deadHTTP {
		err := c.ClearQueue(strconv.Itoa(value))
		if err != nil {
			return err
		}
	}
	return nil
}

// Clear complete queue of the given key/queue name
func (c *Client) ClearQueue(qName string) error {
	err := c.redisCli.Del(c.ctx, qName).Err()
	if err != nil {
		return err
	}
	return nil
}

// GetQueue fetches all messages in queue
func (c *Client) GetQueue(qname string) []InputMsg {
	// Fetch redis list
	queSlice, err := c.redisCli.LRange(c.ctx, qname, 0, -1).Result()
	if err != nil {
		log.Fatalf("Error fetching queue : %v", err)
	}
	// Unmarshal each redis queue message to input message struct
	var queueStruct []InputMsg
	for _, queue := range queSlice {
		queueStruct = append(queueStruct, Unmarshalmsg(queue))
	}
	return queueStruct
}

// SetQueue marshals the input message struct and save it to redis
func (c *Client) SetQueue(queName string, msg InputMsg) error {
	msgInput, err := Marshalmsg(msg)
	if err != nil {
		return err
	}
	// Set message to given queue name(key)
	err = c.redisCli.RPush(c.ctx, queName, msgInput).Err()
	if err != nil {
		return err
	}
	return nil
}

// Fetch input msg detail
func (c *Client) MsgDetail(qName string, msgName string) InputMsg {
	// fetch all messages available in queue
	msgQueue := c.GetQueue(qName)
	for _, msg := range msgQueue {
		if msg.Name == msgName {
			return msg
		}
	}
	return InputMsg{}
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

// MarshalStruct
func Marshalmsg(msg InputMsg) ([]byte, error) {
	return json.Marshal(msg)
}

// Unmarshalmsg
func Unmarshalmsg(msg string) InputMsg {
	var msgStruct InputMsg
	err := json.Unmarshal([]byte(msg), &msgStruct)
	if err != nil {
		log.Fatalf("Error unmarshalling %v", err)
	}
	return msgStruct
}
