# chiya

## run first node

```
$ chiya -o ${own_ip}
```

## add node

```
$ chiya -p 8080 -c_address ${other_node_ip} -c_port ${other_node_port} -o ${own_ip}
```

## run bench
```
$ curl  -X POST -d '{"target": "target url","rate": request rate,"timeout": time out sec,"time_length": 60}' http://${node_ip}
```

## options

```
  -c_address string
    	cluster address
  -c_port string
    	cluster port
  -c_prot string
    	cluster protocol (default "http")
  -o string
    	own ip (default "localhost")
  -p string
    	bench marker port (default "8080")
```
