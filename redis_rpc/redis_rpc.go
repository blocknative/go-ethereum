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
	id       string
	clientID string
	method   string
}

type response struct {
	id     string
	result interface{}
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
		fmt.Println("Got redis rpc request:", req)

		// Handle method and get result.
		var result interface{}
		switch req.method {
		case "txpool_content":
			result = s.txPoolAPI.Content()
		default:
			err = errUnsupportedMethod
		}
		if err != nil {
			log.Error("Error handling request", "err", err)
			continue
		}

		// Send the result to the client.
		err = sendReply(s.redisClient, req.clientID, req.id, result)
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
	cmd := redis.NewStringCmd(ctx, args...)
	if err := redisClient.Process(ctx, cmd); err != nil {
		if err == redis.Nil {
			return req, errClientQueueEmpty
		}
		return req, err
	}
	reqJSON, err := cmd.Result()
	if err != nil {
		return req, err
	}

	// Unmarshal the request
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return req, err
	}
	return req, nil
}

func sendReply(redisClient *redis.Client, clientID string, id string, result interface{}) error {
	respJSON, err := json.Marshal(&response{
		id:     id,
		result: result,
	})
	if err != nil {
		return err
	}
	respKey := makeResponsesKey(clientID)
	ctx := context.Background()
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
