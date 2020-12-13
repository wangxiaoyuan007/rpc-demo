package client

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"rpc-demo/server"
	"sync"
	"time"
)

type SelectMode int
const (
	RandomSelect SelectMode = 0
	RoundRobinSelect SelectMode = 1
)
type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
}
type MultiServersDiscovery struct {
	r *rand.Rand
	mu sync.Mutex
	servers []string
	index int
}
type Xclient struct {
	mu sync.Mutex
	d Discovery
	opt *server.Option
	mode SelectMode
	clients map[string] *Client
}
func NewMultiServersDiscovery(servers []string) *MultiServersDiscovery  {
	d := &MultiServersDiscovery{
		servers: servers,
		r: rand.New(rand.NewSource(time.Now().UnixNano())),

	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d

}
func (m * MultiServersDiscovery) Refresh() error {return nil}

func (m * MultiServersDiscovery)Update(servers []string) error  {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func (m * MultiServersDiscovery)Get(mode SelectMode) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.servers)
	if n == 0 {
		return "",errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return m.servers[m.r.Intn(n)], nil
	case RoundRobinSelect:
		s := m.servers[m.index]
		m.index = (m.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (m * MultiServersDiscovery)GetAll() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// return a copy of d.servers
	servers := make([]string, len(m.servers), len(m.servers))
	copy(servers, m.servers)
	return servers, nil
}

func NewXClient(d Discovery, mode SelectMode,opt *server.Option) *Xclient {
	return &Xclient{
		d: d,
		mode: mode,
		opt: opt,
		clients: make(map[string]*Client),
	}
}

func (xc *Xclient)Close() error  {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key,client := range xc.clients {
		_ =client.Close()
		delete(xc.clients, key)
	}
	return nil
}

func (xc *Xclient) dial(rpcAddr string) (*Client,error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	//先判断缓存中有没有
	client,ok := xc.clients[rpcAddr]
	if ok && !client.IsValiable() {
		_ = client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	//缓存没有
	if client == nil {
		var err error
		client, err = Dial(rpcAddr, xc.opt)
		if err != nil {
			return nil,err
		}
		xc.clients[rpcAddr] = client
	}
	return client,nil
}

func (xc *Xclient) call(rpcAddr string, ctx context.Context, serviceMethod string,args,
	reply interface{})error  {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	 return client.Call(ctx,serviceMethod,args,reply)
}

func (xc *Xclient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error  {
	rpcAddr ,err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

//广播，所有服务实例并发执行，如果一个错误则返回错误，如果全部成功返回一个成功
func (xc *Xclient)Broadcast(ctx context.Context, serviceMethod string, args,reply interface{}) error  {
	servers ,err := xc.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex //protect e and replyDone
	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	var e error
	for _, rpcAddr := range servers{
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var cloneReply interface{}
			if reply != nil {
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr,ctx,serviceMethod,args,cloneReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}

			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
			mu.Unlock()

		}(rpcAddr)

	}
	wg.Wait()
	return e
}