package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type Node struct {
	Host     *string
	Port     *string
	Protocol *string
}

func (n Node) String() string {
	if n.Protocol == nil {
		return fmt.Sprintf("http://%s:%s", *n.Host, *n.Port)
	}
	return fmt.Sprintf("%s://%s:%s", *n.Protocol, *n.Host, *n.Port)
}

var client = &http.Client{}

type Bench struct {
	Target     *string  `json:"target"`
	Rate       *float64 `json:"rate"`
	TimeOut    *int64   `json:"timeout"`
	TimeLength *int64   `json:"time_length"`
}

func fetchURL(wg *sync.WaitGroup, q chan string, r chan bool) {
	// 注意点: ↑これポインタな。
	defer wg.Done()
	for {
		url, ok := <-q // closeされると ok が false になる
		if !ok {
			return
		}
		resp, err := client.Get(url)
		if err != nil {
			r <- false
		} else {
			ioutil.ReadAll(resp.Body)
			r <- true
		}

	}
}

func (b *Bench) Do() (*Result, error) {
	if b.Target == nil {
		return nil, errors.New("no target")
	}
	if b.Rate == nil {
		f := float64(0)
		b.Rate = &f
	}
	if b.TimeOut == nil {
		t := int64(10)
		b.TimeOut = &t
	}
	if b.TimeLength == nil {
		t := int64(10)
		b.TimeLength = &t
	}
	client.Timeout = time.Duration(*b.TimeOut) * time.Millisecond

	result := &Result{}

	timeout := time.After(time.Duration(*b.TimeLength) * time.Second)
	r := 1 / *b.Rate * float64(time.Second)
	tick := time.Tick(time.Duration(int64(r)))

	var wg sync.WaitGroup

	q := make(chan string, 16)
	resultQueue := make(chan bool, 255)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go fetchURL(&wg, q, resultQueue)
	}

	go func() {
		for {
			if b := <-resultQueue; !b {
				result.FailCount++
			}
			result.RequestCount++
		}
	}()

	for {
		select {
		case <-timeout:
			close(q)
			return result, nil
		case <-tick:
			if len(q) == 16 {
				continue
			}
			q <- *b.Target
		default:
		}
	}
}

type Result struct {
	RequestCount int `json:"request_count"`
	FailCount    int `json:"fail_count"`
}
