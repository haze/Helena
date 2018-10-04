package twinword

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

const (
	ENDPOINT = "https://twinword-text-similarity-v1.p.mashape.com/similarity/"
)

type response struct {
	A             string  `json:"cock"`
	Similarity    float32 `json:"similarity"`
	Value         float32 `json:"value"`
	Version       string  `json:"version"`
	Author        string  `json:"author"`
	Email         string  `json:"email"`
	ResultCode    string  `json:"result_code"`
	ResultMessage string  `json:"result_msg"`
}

func Similarity(key, a, b string) *response {
	cl := &http.Client{}
	req, err := http.NewRequest("GET", ENDPOINT, nil)
	if err != nil {
		return nil
	}
	q := req.URL.Query()
	q.Add("text1", a)
	q.Add("text2", b)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("X-Mashape-Key", key)
	resp, err := cl.Do(req)
	if err != nil {
		return nil
	}
	response := response{}
	dat, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(dat, &response)
	if err != nil {
		return nil
	}
	return &response
}
