package deadletterqueue

import (
	"context"
	"encoding/json"
	"fmt"
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
	// Request message
	reqMsgOrd = InputMsg{
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
	// mock to set reqMsg for AddMessage call
	mock.ExpectSet("ReqQueue", structToJson([]InputMsg{reqMsgOrd}), 0).SetVal("OK")

	err := cli.AddMessage(reqMsgOrd)
	assert.Nil(t, err)
}

func TestDeleteReqMsg(t *testing.T) {
	// Request message
	reqMsgSess = InputMsg{
		Name:      "Post session token",
		Url:       "https://api.kite.trade/session/token",
		ReqMethod: "POST",
		PostParam: map[string]interface{}{
			"api_key":       "api_key",
			"request_token": "request_token",
			"checksum":      "checksum",
		},
		Headers: map[string]interface{}{
			"x-kite-version": 3,
		},
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

// structToString parses struct to json for redis mock
func structToJson(msg []InputMsg) []byte {
	jsonMessage, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("%v", err)
	}
	return jsonMessage
}
