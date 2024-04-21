package redis_rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
)

const (
	group = "geth"

	acceptTimeout = 5

	keySeparator = ":"
	keyRoot      = "rpc"
	keyRequests  = "requests"
	keyResponses = "responses"
)

var (
	errClientQueueEmpty  = errors.New("queue empty")
	errUnsupportedMethod = errors.New("unsupported method")
)

type request struct {
	Id       string `json:"id"`
	ClientID string `json:"clientID"`
	Method   string `json:"method"`
}

type response struct {
	Id     string      `json:"id"`
	Result interface{} `json:"result"`
}

type Service struct {
	redisClient *redis.Client
	backend     ethapi.Backend

	txPoolAPI *ethapi.TxPoolAPI

	stopCh chan struct{}
	doneCh chan struct{}
}

func New(redisClient *redis.Client, backend ethapi.Backend) *Service {
	return &Service{
		redisClient: redisClient,
		backend:     backend,

		txPoolAPI: ethapi.NewTxPoolAPI(backend),

		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (s *Service) Start() {
	go s.processRequests()
}

func (s *Service) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *Service) processRequests() {
	for {
		// Stop if we're shutting down.
		select {
		case <-s.stopCh:
			close(s.doneCh)
			return
		default:
		}

		// Wait for the next request
		req, err := waitForRequest(s.redisClient, group)
		if err != nil {
			if !errors.Is(err, errClientQueueEmpty) {
				log.Error("Error waiting for next request", "err", err)
			}
			continue
		}
		fmt.Println("Got redis rpc request:", req.Method)

		// Handle method and get result.
		var result interface{}
		switch req.Method {
		case "txpool_content":
			fmt.Println("Got redis rpc request for mempool")
			result = s.txPoolAPI.Content()
		default:
			err = errUnsupportedMethod
		}
		if err != nil {
			log.Error("Error handling request", "err", err)
			continue
		}

		// Send the result to the client.
		fmt.Println("redis rpc sending reply")
		err = sendReply(s.redisClient, req.ClientID, req.Id, result)
		if err != nil {
			log.Error("Error sending reply", "err", err)
		}
	}
}

func waitForRequest(redisClient *redis.Client, group string) (request, error) {
	// Create args to call the BRPOP command with our queue keys
	args := []interface{}{"BRPOP", makeRequestsKey(group), acceptTimeout}

	// Get the next available value
	fmt.Println("Waiting for redis rpc request...")
	req := request{}
	ctx := context.Background()
	cmd := redis.NewStringSliceCmd(ctx, args...)
	if err := redisClient.Process(ctx, cmd); err != nil {
		if err == redis.Nil {
			return req, errClientQueueEmpty
		}
		fmt.Println("redis rpc request err1:", err)
		return req, err
	}
	brpopResp, err := cmd.Result()
	if err != nil {
		fmt.Println("redis rpc request err2:", err)
		return req, err
	}
	if len(brpopResp) != 2 {
		return req, errors.New("invalid BRPOP response")
	}
	reqJSON := brpopResp[1]
	fmt.Println("Got redis rpc request:", string(reqJSON))

	// Unmarshal the request
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		fmt.Println("redis rpc request err3:", err)
		return req, err
	}
	fmt.Println("redis rpc request:", req)
	return req, nil
}

func sendReply(redisClient *redis.Client, clientID string, id string, result interface{}) error {
	respJSON, err := json.Marshal(&response{
		Id:     id,
		Result: result,
	})
	if err != nil {
		return err
	}
	respKey := makeResponsesKey(clientID)
	ctx := context.Background()
	fmt.Println("pushing redis rpc response", respKey, string(respJSON))
	cmd := redisClient.Do(ctx, "LPUSH", respKey, respJSON)
	err = redisClient.Process(ctx, cmd)
	if err != nil {
		return err
	}
	return nil
}

func makeResponsesKey(clientID string) string {
	return strings.Join([]string{keyRoot, keyResponses, clientID}, keySeparator)
}

func makeRequestsKey(group string) string {
	return strings.Join([]string{keyRoot, keyRequests, group}, keySeparator)
}
