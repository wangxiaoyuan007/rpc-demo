package main

import (
	"log"
	"sync"
	"time"
)



type NetWork struct {
	mu sync.Mutex
	reliable bool
	longDelay bool
	longrecording bool //延迟回应
	ends map[interface{}] *ClientEnd //客户端名字
	enabled  map[interface{}] bool
	server map[interface{}] *Server
	connections map[interface{}]interface{}
	endch chan reqMsg
}


func makeNetWork() *NetWork  {
	rn := &NetWork{
		reliable: true,
		ends: map[interface{}]*ClientEnd{},
		enabled: map[interface{}]bool{},
		server: map[interface{}] *Server{},
		endch: make(chan reqMsg),
		connections: map[interface{}]interface{}{},
	}

	go func() {
		for xreq := range rn.endch{
			go rn.ProcessReq(xreq)
			
		}
	}()
	return rn
}

func (rn *NetWork) ProcessReq(req reqMsg) {
	enable, serverName, server, _, _ := rn.ReadEndNameInfo(req.endName)
	if enable && serverName != nil && server != nil {
		ech := make(chan rspMsg)
		go func() {
			r := server.dispatch(req)
			ech <- r
		}()
		var rsp rspMsg
		rspOk := false
		serverDead := false
		for rspOk == false && serverDead == false{
			select {
			case rsp = <- ech:
				rspOk = true
			case <- time.After(100 * time.Millisecond):
				serverDead = rn.IsServerClose(req.endName, serverName, server)
			}
		}
		serverDead = rn.IsServerClose(req.endName, serverName, server)
		if rspOk == false || serverDead == true{
			req.rsplych <- rspMsg{false,nil}
		} else {
			req.rsplych <- rsp
		}
	}


}

func (rn *NetWork) ReadEndNameInfo(endname interface{}) (enable bool,serverName interface{}, server *Server, reliabl bool, longrecording bool) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	enable = rn.enabled[endname]
	serverName = rn.connections[endname]
	if serverName != nil {
		server = rn.server[serverName]
	}
	reliabl = rn.reliable
	longrecording = rn.longrecording

	return enable, serverName, server,reliabl,longrecording

}
func (rn *NetWork) Connect(endName interface{}, serverName interface{})  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.connections[endName] = serverName
}

func (rn *NetWork) Enable(endName interface{},enable bool)  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.enabled[endName] = enable
}

func (rn *NetWork) Reliable(Yes bool)  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.reliable = Yes
}

func (rn *NetWork) LongDelay(Yes bool)  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.longDelay = Yes
}

func (rn *NetWork) DeleteServer(serverName interface{})  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.server[serverName] = nil
}

//连接客户端服务端
func (rn *NetWork) MakeEnd(endName interface{}) *ClientEnd {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if _, ok := rn.ends[endName];ok {
		log.Fatalf("Make end : %v already exist\n", endName)
		return nil
	}
	e := &ClientEnd{
		endName: endName,
		ch: rn.endch,
	}
	rn.ends[endName] = e
	rn.enabled[endName] = false
	rn.connections[endName] = nil
	return e
}

func (rn *NetWork) IsServerClose(endName interface{}, serverName interface{}, server *Server)  bool{
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if !rn.enabled[endName] || rn.server[serverName] != server {
		return true
	}
	return false
}
func (rn *NetWork)GetCount(serverName interface{}) int  {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	svr := rn.server[serverName]
	return svr.GetCount()
}

func (rn *NetWork) AddServer(serverName string, rs *Server) {
	rn.server[serverName] = rs
}