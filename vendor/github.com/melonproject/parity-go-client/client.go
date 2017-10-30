package parityclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	addr string
}

func NewClient(addr string) *Client {
	return &Client{addr}
}

type Request struct {
	Id      int         `json:"id"`
	Method  string      `json:"method"`
	Jsonrpc string      `json:"jsonrpc"`
	Params  interface{} `json:"params"`
}

type Response struct {
	Id      int             `json:"id"`
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
}

func (c *Client) request(method string, in, out interface{}) error {

	reqBody := Request{
		Id:      1,
		Jsonrpc: "2.0",
		Method:  method,
		Params:  in,
	}

	client := &http.Client{}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshall post body: %v", err)
	}

	body := bytes.NewBuffer(data)

	addr := fmt.Sprintf("http://%s", c.addr)

	req, err := http.NewRequest("POST", addr, body)
	if err != nil {
		return fmt.Errorf("failed to post request on %s: %v", addr, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do the request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d different from 200", resp.StatusCode)
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response Response
	err = json.Unmarshal(data, &response)
	if err != nil {
		return fmt.Errorf("failed to unmarshall response: %v", err)
	}

	err = json.Unmarshal(response.Result, out)
	if err != nil {
		return fmt.Errorf("failed to unmarshall result: %v", err)
	}

	return nil
}

type PeersResponse struct {
	Active    int
	Connected int
	Max       int
}

func (c *Client) Peers() (*PeersResponse, error) {
	var resp PeersResponse
	err := c.request("parity_netPeers", nil, &resp)

	return &resp, err
}
