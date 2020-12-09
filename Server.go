package main

import (
	"log"
	"strings"
	"sync"
)

type Server struct {
	mu sync.Mutex
	services map[string] *Service
	count int
}

func MakeServer() *Server  {
	rs := &Server{}
	rs.services =  map[string] *Service{}
	return rs
}

func (rs *Server) AddService(svc *Service)  {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.services[svc.name] = svc
}

func (rs *Server) dispatch(req reqMsg) rspMsg{
	rs.mu.Lock()
	rs.count++
	dot := strings.LastIndex(req.svcMeth, ".")
	serviceNmae := req.svcMeth[:dot]
	methodName := req.svcMeth[dot + 1:]
	service,ok := rs.services[serviceNmae]
	rs.mu.Unlock()
	if ok {
		return service.Dispatch(methodName,req)
	} else {
		choices := [] string{}
		for k,_ := range rs.services {
			choices = append(choices, k)
		}
		log.Fatalf("service.dispatch:unknown service %v in %v.%v; expecting one of %v \n",
		serviceNmae, serviceNmae,methodName,choices)
		return rspMsg{false,nil}
	}
}

func (rs *Server) GetCount() int  {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.count
}
