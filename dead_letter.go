package deadletterqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// InputMsg represents input message to be added to queue
type InputMsg struct {
	Name      string
	Url       string
	ReqMethod string
	PostParam map[string]interface{}
	Headers   map[string]interface{}
}

// Client represents interface for redis queue
type Client struct {
	redisCli  *redis.Client
	queueName string
	ctx       context.Context
	deadHTTP  []int
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
	queueMsg := append(c.GetQueue(c.queueName), message)
	return c.SetQueue(c.queueName, queueMsg)
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
	queueData := c.GetQueue(qName)
	if len(queueData) > 0 {
		for _, queue := range queueData {
			c.RawExecute(queue, qName)
		}
	} else {
		log.Printf("No messages in %v queue to execute", qName)
	}
}

// RawExecute performs the HTTP request based on request params
func (c *Client) RawExecute(msgParam InputMsg, qName string) {
	// Add all POST or PUT params as url.values map
	postData := url.Values{}
	if msgParam.PostParam != nil {
		for key, value := range msgParam.PostParam {
			valueStr := fmt.Sprintf("%v", value)
			postData.Add(key, valueStr)
		}
	}
	var postBody io.Reader
	if msgParam.ReqMethod == "POST" || msgParam.ReqMethod == "PUT" {
		// convert post params map into “URL encoded” form
		paramsEncoded := postData.Encode()
		postBody = bytes.NewReader([]byte(paramsEncoded))
	}
	req, _ := http.NewRequest(msgParam.ReqMethod, msgParam.Url, postBody)

	// Add all request headers to the http request
	reqHeader := http.Header{}
	if msgParam.Headers != nil {
		for name, value := range msgParam.Headers {
			valueStr := fmt.Sprintf("%v", value)
			reqHeader.Add(name, valueStr)
		}
		req.Header = reqHeader
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request : %v", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body %v", err)
	}
	// Store response body data
	c.MessageResponse(msgParam.Name, string(body))

	c.HandleDeadQueue(res, msgParam, qName)

	// Add response body data to redis to be fetched by another worker
	// To:do- Have better key hash to store successful response
	successKey := msgParam.Url + msgParam.ReqMethod
	err = c.redisCli.Set(c.ctx, successKey, string(body), 0).Err()
	if err != nil {
		log.Printf("Error updating successKey for request : %v", err)
	}
}

// MessageResponse stores response body of the request body
func (c *Client) MessageResponse(msgName string, response string) {
	err := c.redisCli.Set(c.ctx, msgName, response, 0).Err()
	if err != nil {
		log.Printf("Error updating response update for the req message %s", msgName)
	}
}

// HandleDeadQueue creates/update dead queue to retry later
func (c *Client) HandleDeadQueue(res *http.Response, msgParam InputMsg, qName string) {
	// Create/add dead letter queue based on user input for deadHTTP
	if Find(c.deadHTTP, res.StatusCode) {
		// Alert user with failed status for HTTP request
		log.Printf("Request msg %s, failed with status %s", msgParam.Name, res.Status)
		// Add failed messages to dead letter queue
		qkey := strconv.Itoa(res.StatusCode)
		deadMsg := append(c.GetQueue(qkey), msgParam)
		err := c.SetQueue(qkey, deadMsg)
		if err != nil {
			log.Printf("Error adding dead queue : %v", err)
		}
	}
	// Delete executed message from queue
	newQueue := RemoveIndex(c.GetQueue(qName), 0)
	err := c.SetQueue(qName, newQueue)
	if err != nil {
		log.Printf("Error adding to %s queue : %v", qName, err)
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
	var filterQueue []InputMsg
	for _, value := range c.GetQueue(queName) {
		if value.Name != msgName {
			filterQueue = append(filterQueue, value)
		}
	}
	return c.SetQueue(queName, filterQueue)
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
	var queueSlice []InputMsg
	val, err := c.redisCli.Get(c.ctx, qname).Result()
	if err != nil {
		// handle empty redis GET
		if val != "" {
			log.Printf("Error fetching queue : %v", err)
		}
	}
	err = json.Unmarshal([]byte(val), &queueSlice)
	return queueSlice
}

// SetQueue marshals the input message struct and save it to redis
func (c *Client) SetQueue(queName string, msg []InputMsg) error {
	jsonMessage, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	// Set message to given queue name(key)
	err = c.redisCli.Set(c.ctx, queName, jsonMessage, 0).Err()
	if err != nil {
		return err
	}
	return nil
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
