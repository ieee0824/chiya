package util

import (
	"errors"
	"fmt"
	"log"
	"net/http"
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

type Bench struct {
	Target     *string  `json:"target"`
	Rate       *float64 `json:"rate"`
	TimeOut    *int64   `json:"timeout"`
	TimeLength *int64   `json:"time_length"`
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
	var client = &http.Client{
		Timeout: time.Duration(*b.TimeOut) * time.Millisecond,
	}
	result := &Result{}

	timeout := time.After(time.Duration(*b.TimeLength) * time.Second)
	r := 1 / *b.Rate * float64(time.Second)
	tick := time.Tick(time.Duration(int64(r)))

	for {
		select {
		case <-timeout:
			return result, nil
		case <-tick:
			_, err := client.Get(*b.Target)
			if err != nil {
				log.Println(err)
				result.FailCount++
			} else {
				result.RequestCount++
			}
		}
	}
}

type Result struct {
	RequestCount int `json:"request_count"`
	FailCount    int `json:"fail_count"`
}
