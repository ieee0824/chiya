package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ieee0824/chiya/util"
)

var (
	PORT             *string
	CLUSTER_ADDRESS  *string
	CLUSTER_PORT     *string
	CLUSTER_PROTOCOL *string
	OWN_HOST         *string
)
var own = &util.Node{}

func init() {
	log.SetFlags(log.Llongfile)
	OWN_HOST = flag.String("o", "localhost", "own ip")
	PORT = flag.String("p", "8080", "bench marker port")
	CLUSTER_ADDRESS = flag.String("c_address", "", "cluster address")
	CLUSTER_PORT = flag.String("c_port", "", "cluster port")
	CLUSTER_PROTOCOL = flag.String("c_prot", "http", "cluster protocol")
	flag.Parse()
}

func initialize() {
	own.Host = OWN_HOST
	own.Port = PORT
	own.Protocol = CLUSTER_PROTOCOL
	if CLUSTER_ADDRESS == nil {
		CLUSTER_ADDRESS = nil
		CLUSTER_PORT = nil
		return
	}
	if *CLUSTER_ADDRESS == "" {
		CLUSTER_ADDRESS = nil
		CLUSTER_PORT = nil
		return
	}
	node := &util.Node{
		CLUSTER_ADDRESS,
		CLUSTER_PORT,
		CLUSTER_PROTOCOL,
	}
	a := NewAddPacket()
	a.Node = own
	nodeTable[node.String()] = node
	if err := add(a); err != nil {
		log.Fatalln(err)
	}
}

type addPacket struct {
	TTL  *int       `json:"ttl"`
	Node *util.Node `json:"node"`
}

func NewAddPacket() *addPacket {
	r := &addPacket{
		pInt(10),
		nil,
	}
	return r
}

var client = &http.Client{
	Timeout: 10 * time.Second,
}

var nodeTable = map[string]*util.Node{}

// /add apiを叩く
func add(a *addPacket) error {
	if a == nil {
		return errors.New("add info is nil")
	}
	log.Println("run add func")
	packet, err := json.Marshal(a)
	if err != nil {
		return err
	}
	log.Println(nodeTable)
	for k, v := range nodeTable {
		if k != a.Node.String() {
			log.Println(v.String() + "/add")
			req, err := http.NewRequest("POST", v.String()+"/add", bytes.NewReader(packet))
			if err != nil {
				return err
			}

			if _, err := client.Do(req); err != nil {
				return err
			}
		}
	}
	return nil
}

func check(n *util.Node) error {
	log.Println("run check")
	if n == nil {
		return errors.New("node is nil")
	}
	o, err := json.Marshal(own)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", n.String()+"/check", bytes.NewReader(o))
	if err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		_, err := client.Do(req)
		if err == nil {
			return nil
		}
		log.Println("wait 1 sec")
		time.Sleep(1 * time.Second)
	}

	return errors.New("node check fail")
}

func internalError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	bin, _ := json.Marshal(err)
	w.Write(bin)
}

func pInt(i int) *int {
	return &i
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("add handle")
	// addPacketを受け取ったらTTLを減らす
	bin, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	addPacket := &addPacket{}
	if err := json.Unmarshal(bin, addPacket); err != nil {
		internalError(w, err)
		return
	}
	node := addPacket.Node
	if node != nil {
		nodeTable[node.String()] = node
	}

	if err := check(node); err != nil {
		internalError(w, err)
		return
	}

	addPacket.TTL = pInt(*addPacket.TTL - 1)
	if *addPacket.TTL == 0 {
		return
	}
	if err := add(addPacket); err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("check handler")
	bin, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	var node = &util.Node{}

	if err := json.Unmarshal(bin, node); err != nil {
		internalError(w, err)
		return
	}
	nodeTable[node.String()] = node
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	bin, _ := json.Marshal(nodeTable)

	w.Header().Set("Content-Type", "application/json")
	w.Write(bin)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("It works"))
}

func transferBench(rule *util.Bench) []util.Result {
	var wg sync.WaitGroup
	ping()
	ret := []util.Result{}
	if rule == nil {
		return nil
	}
	q := make(chan util.Node, len(nodeTable)*2)
	resultQ := make(chan util.Result, len(nodeTable)*2)
	requestBody, _ := json.Marshal(rule)

	for i := 0; i < len(nodeTable)+1; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, q chan util.Node, r chan util.Result) {
			defer wg.Done()
			for {
				node, ok := <-q
				if !ok {
					return
				}
				req, err := http.NewRequest("PORT", node.String()+"/api/bench", bytes.NewReader(requestBody))
				if err != nil {
					log.Println(err)
					var status = false
					resultQ <- util.Result{Status: &status}
				}
				resp, err := client.Do(req)
				if err != nil {
					log.Println(err)
					var status = false
					resultQ <- util.Result{Status: &status}
				}
				defer resp.Body.Close()
				respBody, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					var status = false
					resultQ <- util.Result{Status: &status}
				}
				result := &util.Result{}
				if err := json.Unmarshal(respBody, result); err != nil {
					log.Println(string(respBody))
					log.Println(err)
					var status = false
					resultQ <- util.Result{Status: &status}
				}
				var status = true
				result.Status = &status
				resultQ <- *result
			}
		}(&wg, q, resultQ)
	}

	for _, v := range nodeTable {
		q <- *v
	}
	q <- *own
	close(q)

	for {
		ret = append(ret, <-resultQ)
		if len(ret) >= len(nodeTable)+1 {
			return ret
		}
	}

}

var pingClient = http.Client{
	Timeout: 10 * time.Second,
}

func pingWorker(wg *sync.WaitGroup, q chan util.Node) {
	defer wg.Done()
	for {
		node, ok := <-q
		if !ok {
			return
		}
		_, err := pingClient.Get(node.String())
		if err != nil {
			log.Println(node.String() + " is delete")
			delete(nodeTable, node.String())
		}
	}
}

// 動いてるノードを調べる
// 反応がない場合nodeを削除する
func ping() {
	var wg sync.WaitGroup
	q := make(chan util.Node, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go pingWorker(&wg, q)
	}
	for _, v := range nodeTable {
		q <- *v
	}
	close(q)
	wg.Wait()
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	bin, _ := json.Marshal(true)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bin)
}

func clusterBenchHandler(w http.ResponseWriter, r *http.Request) {
	bin, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	var bench = &util.Bench{}
	if err := json.Unmarshal(bin, bench); err != nil {
		log.Println("\n" + string(bin))
		log.Println(err)
		internalError(w, err)
		return
	}
	results := transferBench(bench)
	w.Header().Set("Content-Type", "application/json")
	respBody, _ := json.Marshal(results)
	w.Write(respBody)
}

func benchAPI(w http.ResponseWriter, r *http.Request) {
	bin, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	var bench = &util.Bench{}
	if err := json.Unmarshal(bin, bench); err != nil {
		log.Println("\n" + string(bin))
		log.Println(err)
		internalError(w, err)
		return
	}

	result, err := bench.Do()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	respBody, _ := json.Marshal(result)
	w.Write(respBody)
}

func benchHandler(w http.ResponseWriter, r *http.Request) {
	bin, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	var bench = &util.Bench{}
	if err := json.Unmarshal(bin, bench); err != nil {
		log.Println("\n" + string(bin))
		log.Println(err)
		internalError(w, err)
		return
	}

	result, err := bench.Do()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(result.String()))
}

func main() {
	fin := make(chan bool)
	go func() {
		initialize()
		fin <- true
	}()
	client.Timeout = 1 * time.Hour
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/check", checkHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/bench", benchHandler)
	http.HandleFunc("/api/bench", benchAPI)
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/cluster", clusterBenchHandler)
	http.ListenAndServe(":"+*PORT, nil)
	<-fin
}
