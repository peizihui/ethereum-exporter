package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
)

func args(in ...interface{}) []interface{} {
	out := []interface{}{}
	for _, i := range in {
		out = append(out, i)
	}
	return out
}

type Etherscan struct {
	addr string
}

func (e *Etherscan) BlockNumber() (*big.Int, error) {
	resp, err := http.Get(e.addr)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ensureOk(resp)
	if err != nil {
		return nil, err
	}

	var res string
	if err = json.Unmarshal(*data, &res); err != nil {
		return nil, err
	}

	return hexToBigInt(res)
}

type EthClient struct {
	addr string
}

func NewEthClient(addr string) *EthClient {
	return &EthClient{addr}
}

type RPCRequest struct {
	Id      int         `json:"id"`
	Method  string      `json:"method"`
	Jsonrpc string      `json:"jsonrpc"`
	Params  interface{} `json:"params"`
}

type RPCResult struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
}

func (e *EthClient) rpcCall(method string, in, out interface{}) error {
	if in == nil {
		in = []interface{}{}
	}

	reqBody := RPCRequest{
		Id:      1,
		Jsonrpc: "2.0",
		Method:  method,
		Params:  in,
	}

	client := &http.Client{}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(reqData)

	req, err := http.NewRequest("POST", e.addr, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err := ensureOk(resp)
	if err != nil {
		return err
	}

	err = json.Unmarshal(*data, out)
	if err != nil {
		return fmt.Errorf("failed to unmarshall result: %v", err)
	}

	return err
}

func ensureOk(resp *http.Response) (*json.RawMessage, error) {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d different from 200: %s", resp.StatusCode, string(data))
	}

	var res RPCResult

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}

	return &res.Result, nil
}

func hexToBigInt(data string) (*big.Int, error) {
	blockInt64, err := strconv.ParseInt(data, 0, 64)
	if err != nil {
		return nil, err
	}

	return big.NewInt(blockInt64), nil
}

func (e *EthClient) PeerCount() (int64, error) {
	var peers string
	if err := e.rpcCall("net_peerCount", nil, &peers); err != nil {
		return 0, err
	}

	return strconv.ParseInt(peers, 0, 64)
}

func (e *EthClient) Chain() (string, error) {
	var chain string
	err := e.rpcCall("parity_chain", nil, &chain)
	return chain, err
}

func (e *EthClient) BlockNumber() (*big.Int, error) {
	var block string
	if err := e.rpcCall("eth_blockNumber", nil, &block); err != nil {
		return nil, err
	}

	return hexToBigInt(block)
}

type Block struct {
	Timestamp    *time.Time
	Transactions int
	GasLimit     *big.Int
}

func (e *EthClient) BlockByNumber(num *big.Int) (*Block, error) {
	hash := fmt.Sprintf("0x%x", num)

	var result error

	var raw map[string]interface{}
	if err := e.rpcCall("eth_getBlockByNumber", args(hash, true), &raw); err != nil {
		return nil, err
	}

	block := &Block{}

	if timestampHex, ok := raw["timestamp"]; ok {
		timestamp, err := hexToBigInt(timestampHex.(string))
		if err != nil {
			result = multierror.Append(result, err)
		}

		tm := time.Unix(timestamp.Int64(), 0)
		block.Timestamp = &tm
	} else {
		result = multierror.Append(result, fmt.Errorf("timestamp field not found"))
	}

	if transactionsRaw, ok := raw["transactions"]; ok {
		if transactions, ok := transactionsRaw.([]interface{}); ok {
			block.Transactions = len(transactions)
		} else {
			result = multierror.Append(result, fmt.Errorf("Transaction field found but not an interface"))
		}
	} else {
		result = multierror.Append(result, fmt.Errorf("transactions field not found"))
	}

	if gasLimitRaw, ok := raw["gasLimit"]; ok {
		gasLimit, err := hexToBigInt(gasLimitRaw.(string))
		if err != nil {
			result = multierror.Append(result, err)
		}

		block.GasLimit = gasLimit
	} else {
		result = multierror.Append(result, fmt.Errorf("gaslimit field not found"))
	}

	return block, nil
}

type RpcSync struct {
	CurrentBlock        *big.Int
	HighestBlock        *big.Int
	StartingBlock       *big.Int
	WarpChunksAmount    *big.Int
	WarpChunksProcessed *big.Int
}

func (e *EthClient) Syncing() (*RpcSync, error) {
	var raw interface{}
	if err := e.rpcCall("eth_syncing", nil, &raw); err != nil {
		return nil, err
	}

	_, ok := raw.(bool)
	if ok {
		return nil, nil
	}

	type rpcSync struct {
		CurrentBlock, HighestBlock, StartingBlock string
		WarpChunksProcessed, WarpChunksAmount     string
	}

	var res rpcSync
	if err := mapstructure.Decode(raw, &res); err != nil {
		return nil, err
	}

	currentBlock, err := hexToBigInt(res.CurrentBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current block as big.Int: %s", res.CurrentBlock)
	}

	highestBlock, err := hexToBigInt(res.HighestBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse highest block as big.Int: %s", res.HighestBlock)
	}

	startingBlock, err := hexToBigInt(res.StartingBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse starting block as big.Int: %s", res.HighestBlock)
	}

	warpChunksAmount, err := hexToBigInt(res.WarpChunksAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse warpChunksAmount as big.Int: %s", res.HighestBlock)
	}

	warpChunksProcessed, err := hexToBigInt(res.WarpChunksProcessed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse warpChunksProcessed as big.Int: %s", res.HighestBlock)
	}

	sync := &RpcSync{
		HighestBlock:        highestBlock,
		CurrentBlock:        currentBlock,
		StartingBlock:       startingBlock,
		WarpChunksAmount:    warpChunksAmount,
		WarpChunksProcessed: warpChunksProcessed,
	}

	return sync, nil
}
