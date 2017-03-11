package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
var own = &util.Node{}

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

func benchHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("start bench")
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
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/check", checkHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/bench", benchHandler)
	http.ListenAndServe(":"+*PORT, nil)
	<-fin
}
