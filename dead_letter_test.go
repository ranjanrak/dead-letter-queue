package deadletterqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/assert"
)

var (
	db         *redis.Client
	mock       redismock.ClientMock
	cli        Client
	reqMsgOrd  InputMsg
	reqMsgSess InputMsg
)

// MockRedis sets redis dbclient and mockclient
func MockRedis() {
	db, mock = redismock.NewClientMock()
	cli = Client{
		redisCli:  db,
		queueName: "ReqQueue",
		ctx:       context.TODO(),
		deadHTTP:  []int{400, 429, 502},
	}
}

func TestAddMessage(t *testing.T) {
	// Initialize the mock redis
	MockRedis()

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
	reqMsgOrd = InputMsg{
		Name:      "Place TCS Order",
		Url:       "https://api.kite.trade/orders/regular",
		ReqMethod: "POST",
		PostParam: postParam,
		Headers:   headers,
	}
	// mock to set reqMsg for AddMessage call
	mock.ExpectSet("ReqQueue", structToJson([]InputMsg{reqMsgOrd}), 0).SetVal("OK")

	err := cli.AddMessage(reqMsgOrd)
	assert.Nil(t, err)
}

func TestDeleteReqMsg(t *testing.T) {
	// Add post params
	postParam := url.Values{}
	postParam.Add("api_key", "api_key")
	postParam.Add("request_token", "request_token")
	postParam.Add("checksum", "checksum")

	// Add request header
	var headers http.Header = map[string][]string{}
	headers.Add("x-kite-version", "3")

	// Request message
	reqMsgSess = InputMsg{
		Name:      "Post session token",
		Url:       "https://api.kite.trade/session/token",
		ReqMethod: "POST",
		PostParam: postParam,
		Headers:   headers,
	}
	// Add mock to add both reqMsg and reqMsgSess in ReqQueue
	mock.ExpectGet("ReqQueue").SetVal(string(structToJson([]InputMsg{reqMsgOrd, reqMsgSess})))
	// Inspect only reqMsgSess message is left post removal of "Place TCS Order" message
	// from ReqQueue
	mock.ExpectSet("ReqQueue", structToJson([]InputMsg{reqMsgSess}), 0).SetVal("OK")

	err := cli.DeleteReqMsg("Place TCS Order")
	assert.Nil(t, err)
}

func TestDeleteDeadMsg(t *testing.T) {
	// Add Get and Set mock for all dead http key
	mock.ExpectGet("400").SetVal(string(structToJson([]InputMsg{reqMsgOrd, reqMsgSess})))
	mock.ExpectSet("400", structToJson([]InputMsg{reqMsgSess}), 0).SetVal("OK")

	mock.ExpectGet("429").SetVal(string(structToJson([]InputMsg{reqMsgOrd, reqMsgSess})))
	mock.ExpectSet("429", structToJson([]InputMsg{reqMsgSess}), 0).SetVal("OK")

	mock.ExpectGet("502").SetVal(string(structToJson([]InputMsg{reqMsgOrd, reqMsgSess})))
	mock.ExpectSet("502", structToJson([]InputMsg{reqMsgSess}), 0).SetVal("OK")

	err := cli.DeleteDeadMsg("Place TCS Order")
	assert.Nil(t, err)
}

func TestMessageStatus(t *testing.T) {
	// Load mock response
	mockOrders, err := ioutil.ReadFile("./mockdata/orderbook_response.json")
	if err != nil {
		t.Errorf("Error while fetching orderbook_response. %v", err)
	}
	// Mock to fetch request status
	mock.ExpectGet("Fetch order book").SetVal(string(mockOrders))
	// Check response status for executed message
	response, err := cli.MessageStatus("Fetch order book")
	if err != nil {
		fmt.Printf("Error %v", err)
	}
	var mockStruct map[string]interface{}
	json.Unmarshal([]byte(response), &mockStruct)
	//assert
	assert.Equal(t, mockStruct["status"], "success", "Fetch order book request failed.")
}

// structToString parses struct to json for redis mock
func structToJson(msg []InputMsg) []byte {
	jsonMessage, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("%v", err)
	}
	return jsonMessage
}
