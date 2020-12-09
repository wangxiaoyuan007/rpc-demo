package main

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

type JunkArgs struct {
	X int
}

type JunkReply struct {
	X string
}

type JunkServer struct {
	mu sync.Mutex
	log1 [] string
	log2 [] int
}

func (js *JunkServer) Handler1(args string, reply *int)  {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.log1 = append(js.log1, args)
	*reply, _ = strconv.Atoi(args)
}

func (js *JunkServer) Handler2(args int, reply *string)  {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.log2 = append(js.log2, args)
	*reply = "handler2-" + strconv.Itoa(args)
}

func (js *JunkServer) Handler3(args int, reply *int)  {
	js.mu.Lock()
	defer js.mu.Unlock()
	time.Sleep(20 * time.Second)
	*reply = -args
}

func (js *JunkServer) Handler4(args *JunkArgs, reply *JunkReply)  {
	reply.X = "pointer"
}

func (js *JunkServer) Handler5(args *JunkArgs, reply *JunkReply)  {
	reply.X = "no pointer"
}

func TestBasic(t *testing.T)  {
	runtime.GOMAXPROCS(4)
	rn := makeNetWork()
	e := rn.MakeEnd("end1-99")
	js := &JunkServer{}
	svc := makeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer("server99", rs)

	rn.Connect("end1-99","server99")
	rn.Enable("end1-99",true)

	{
		reply := ""
		e.Call("JunkServer.Handler2",111,&reply)
		fmt.Println("reply:--------" + reply)
		if reply != "handler2-111" {
			t.Fatalf("worng reply from Handler2")
		}
	}

	{
		reply := 0
		e.Call("JunkServer.Handler1","9099",&reply)
		if reply != 9099 {
			t.Fatalf("worng reply from Handler1")
		}
	}
}