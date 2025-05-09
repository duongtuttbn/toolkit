package client_pool

import (
	"context"
	"fmt"
	"github.com/chenzhijie/go-web3"
	"github.com/duongtuttbn/toolkit/log"
	"github.com/duongtuttbn/toolkit/model"
	"github.com/duongtuttbn/toolkit/utils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"math/big"
	"strings"
	"sync"
	"time"
)

type (
	ClientPool struct {
		clients []*Client
		counter int
		mu      sync.Mutex
		config  Config
	}

	GetBlockTimeResponse struct {
		Result struct {
			Timestamp string `json:"timestamp"`
		} `json:"result"`
	}
)

const (
	tokenInfoABI         = "[{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"
	liquidityPoolInfoABI = "[{\"inputs\":[],\"name\":\"token0\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"token1\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"
)

func NewBasicClientPool(cfg Config) (*ClientPool, error) {
	rpcUrls := strings.Split(cfg.RpcUrls, ",")
	clients := make([]*Client, len(rpcUrls))
	for i, rpcUrl := range rpcUrls {
		var err error
		clients[i], err = NewClient(rpcUrl, "")
		if err != nil {
			return nil, errors.Wrap(err, "unable to init new client")
		}
	}
	return &ClientPool{
		clients: clients,
		config:  cfg,
	}, nil
}

// GetClient return a client that available for use,
// blocked if there is no client available
func (pool *ClientPool) GetClient() *Client {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for {
		for i := 0; i < len(pool.clients); i++ {
			client := pool.clients[pool.counter]
			pool.counter = (pool.counter + 1) % len(pool.clients)
			if client.IsAvailable() {
				log.Debugf("Use client: %s", client.endpoint)
				return client
			}
		}
		logrus.Infof("all clients are down, sleep for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

// GetClients return all available clients
func (pool *ClientPool) GetClients(numClients int) ([]*Client, error) {
	if numClients > len(pool.clients) {
		return nil, errors.New(fmt.Sprintf("numClients must be less than client pool size: %d", len(pool.clients)))
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for {
		availableClients := pool.countAvailableClients()
		if availableClients < numClients {
			logrus.Infof("Request %d clients but only %d available. Sleep for 1 minute", numClients, availableClients)
			time.Sleep(time.Minute)
			continue
		} else {
			break
		}
	}

	clients := make([]*Client, 0, numClients)
	for len(clients) < numClients {
		client := pool.clients[pool.counter]
		pool.counter = (pool.counter + 1) % len(pool.clients)
		if client.IsAvailable() {
			clients = append(clients, client)
		}
	}
	return clients, nil
}

// GetAllClients return all clients regardless their availability
func (pool *ClientPool) GetAllClients() []*Client {
	return pool.clients
}

func (pool *ClientPool) countAvailableClients() int {
	counter := 0
	for _, client := range pool.clients {
		if client.IsAvailable() {
			counter++
		}
	}
	return counter
}

// RunOp execute a given callback for all clients in the pool
func (pool *ClientPool) RunOp(ctx context.Context, op func(client *Client) error) {
	for ctx.Err() == nil {
		client := pool.GetClient()
		err := op(client)
		if err == nil {
			return
		}
	}
}

// GetLatestBlock return latest block number
func (pool *ClientPool) GetLatestBlock() uint64 {
	for {
		client := pool.GetClient()
		maxBlock, err := client.BlockNumber(context.Background())
		if err != nil {
			client.MarkError(err)
			logrus.Errorf("get max block error: %v", err)
			continue
		}
		return maxBlock
	}
}

func (pool *ClientPool) GetLogs(filterQuery ethereum.FilterQuery, fromBlock, toBlock uint64, numProof int) (
	[]types.Log,
	error,
) {
	if numProof <= 1 {
		return pool.getLogs(filterQuery, fromBlock, toBlock)
	}

	var wg sync.WaitGroup
	wg.Add(numProof)
	results := make([][]types.Log, numProof)
	resultsLocker := sync.Mutex{}
	availableClients, err := pool.GetClients(numProof)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get clients")
	}

	for i := 0; i < numProof; i++ {
		go func(index int) {
			defer wg.Done()
			logs, getLogsError := pool.getLogs(filterQuery, fromBlock, toBlock, availableClients[index])
			if getLogsError != nil {
				err = errors.Wrapf(getLogsError, "getLogs {%d} error", index)
			}
			resultsLocker.Lock()
			results[index] = logs
			resultsLocker.Unlock()
		}(i)
	}

	wg.Wait()
	if err != nil {
		return nil, err
	}

	consistent := true
	for i := 0; i < numProof; i++ {
		if i < numProof-1 {
			consistent = pool.compareListsLogs(results[i], results[i+1])
			if !consistent {
				break
			}
		}
	}
	if consistent {
		return results[0], nil
	} else {
		for i := 0; i < numProof; i++ {
			logrus.Infof(
				"\t[Consistency error trace] Block range: [%d, %d]: Client %s returned %d logs",
				fromBlock,
				toBlock,
				availableClients[i].endpoint,
				len(results[i]),
			)
		}
		return nil, errors.New("Consistency error")
	}
}

func (pool *ClientPool) compareListsLogs(logs1 []types.Log, logs2 []types.Log) (equal bool) {
	if len(logs1) != len(logs2) {
		logrus.Debugf("logs1 (%d items) is not equal to logs2 (%d items)", len(logs1), len(logs2))
		return false
	}

	for i := 0; i < len(logs1); i++ {
		if !pool.compareLogs(logs1[i], logs2[i]) {
			logrus.Debugf("logs1[%d] is not equal to logs2[%d]. \n\tlog1: %v\n\tlog2: %v", i, i, logs1[i], logs2[i])
			return false
		}
	}
	return true
}

func (pool *ClientPool) compareLogs(log1 types.Log, log2 types.Log) (equal bool) {
	return log1.TxHash == log2.TxHash &&
		log1.Index == log2.Index
}

func (pool *ClientPool) getLogs(
	filterQuery ethereum.FilterQuery,
	fromBlock, toBlock uint64,
	specificClient ...*Client,
) ([]types.Log, error) {
	for {
		client := pool.GetClient()
		if fromBlock > toBlock {
			return []types.Log{}, nil
		}
		filterQuery.FromBlock = big.NewInt(int64(fromBlock))
		filterQuery.ToBlock = big.NewInt(int64(toBlock))
		logs, err := client.FilterLogs(context.Background(), filterQuery)
		if err != nil {
			client.MarkError(err)
			logrus.Errorf("Fetch logs [%d to %d] on endpoint %v error: %v", fromBlock, toBlock, client.endpoint, err)
			continue
		}
		if isLogTooLargeError(err) && toBlock > fromBlock {
			midBlockNumber := fromBlock + (toBlock-fromBlock)/2
			log1, err1 := pool.getLogs(filterQuery, fromBlock, midBlockNumber)
			if err1 != nil {
				return nil, err1
			}
			log2, err2 := pool.getLogs(filterQuery, midBlockNumber+1, toBlock)
			if err2 != nil {
				return nil, err2
			}
			logs = append(log1, log2...)
			return logs, nil
		}
		return logs, errors.Wrap(err, "client endpoint: "+client.endpoint)
	}
}

func (pool *ClientPool) BlockTime(blockNumber uint64) uint64 {
	if pool.config.ManualBlockTime {
		return pool.manualBlockTime(blockNumber)
	}
	return pool.rpcBlockTime(blockNumber)
}

func (pool *ClientPool) rpcBlockTime(blockNumber uint64) uint64 {
	for {
		ethClient := pool.GetClient()
		block, err := ethClient.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
		if err != nil {
			logrus.Infof(
				"error requesting blocktime from node, backing off. BlockNumber: %v Endpoint: %v, Err: %v,",
				blockNumber,
				ethClient.endpoint,
				err,
			)
			ethClient.MarkError(err)
			continue
		}
		return block.Time()
	}
}

func (pool *ClientPool) manualBlockTime(blockNumber uint64) uint64 {
	for {
		ethClient := pool.GetClient()
		url := ethClient.endpoint
		body := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_getBlockByNumber",
			"params": []interface{}{
				DecimalToHex(int64(blockNumber)),
				true,
			},
			"id": 0,
		}
		client := resty.New()
		res, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(body).
			SetResult(GetBlockTimeResponse{}).
			Post(url)
		if err != nil {
			logrus.Infof(
				"error manual requesting blocktime from node, backing off. BlockNumber: %v Endpoint: %v, Err: %v,",
				blockNumber,
				ethClient.endpoint,
				err,
			)
			ethClient.MarkError(err)
			continue
		}
		if res.IsError() {
			logrus.Infof(
				"error manual requesting blocktime from node, status code error. BlockNumber: %v Endpoint: %v, Err: %v,",
				blockNumber,
				ethClient.endpoint,
				string(res.Body()),
			)
			ethClient.MarkError(err)
			continue
		}
		data := res.Result().(*GetBlockTimeResponse)
		result, err := HexToInt(data.Result.Timestamp)
		if err != nil {
			logrus.Infof(
				"error manual requesting blocktime from node, hex to int. BlockNumber: %v Endpoint: %v, Err: %v,",
				blockNumber,
				ethClient.endpoint,
				err,
			)
			ethClient.MarkError(err)
			continue
		}
		return uint64(result)
	}

}

func (pool *ClientPool) GetToBlock(fromRange int64, maxToBlock int64) int64 {
	if fromRange < maxToBlock {
		return fromRange
	}
	return maxToBlock
}

func (pool *ClientPool) GetTransactionReceipt(txHash common.Hash) (*types.Receipt, error) {
	for {
		client := pool.GetClient()
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get tx receipt error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		return receipt, nil
	}
}

func (pool *ClientPool) GetTokenInfo(tokenAddress string) (*model.TokenInfo, error) {
	for {
		item := &model.TokenInfo{}
		client := pool.GetClient()
		clientWeb3, err := web3.NewWeb3(client.endpoint)
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("init web 3 client error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		contract, err := clientWeb3.Eth.NewContract(tokenInfoABI, tokenAddress)
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("init contract error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		name, err := contract.Call("name")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract name error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		symbol, err := contract.Call("symbol")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract symbol error: %v", err)
				continue
			} else {
				return nil, err
			}
		}

		decimals, err := contract.Call("decimals")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract decimals error: %v", err)
				continue
			} else {
				return nil, err
			}
		}

		totalSupply, err := contract.Call("totalSupply")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract total supply error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		item.TokenName = name.(string)
		item.TokenSymbol = symbol.(string)
		item.TokenAddress = tokenAddress
		item.ContractDecimals = int64(decimals.(uint8))
		item.TotalSupply = utils.BigIntToFloat(totalSupply.(*big.Int), item.ContractDecimals)
		return item, err
	}
}

func (pool *ClientPool) GetLiquidityPoolInfo(poolAddress string) (*model.LiquidityPoolInfo, error) {
	for {
		item := &model.LiquidityPoolInfo{}
		client := pool.GetClient()
		clientWeb3, err := web3.NewWeb3(client.endpoint)
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("init web 3 client error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		contract, err := clientWeb3.Eth.NewContract(liquidityPoolInfoABI, poolAddress)
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("init contract error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		token0, err := contract.Call("token0")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract name error: %v", err)
				continue
			} else {
				return nil, err
			}
		}
		token1, err := contract.Call("token1")
		if err != nil {
			if isRateLimit(err) {
				client.MarkError(err)
				logrus.Errorf("get contract symbol error: %v", err)
				continue
			} else {
				return nil, err
			}
		}

		item.Token0 = token0.(common.Address).String()
		item.Token1 = token1.(common.Address).String()
		item.PoolAddress = poolAddress
		return item, err
	}
}
