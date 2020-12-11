package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"rpc-demo/codec"
	"rpc-demo/server"
	"sync"
	"time"
)

type ClientResult struct {
	client *Client
	err error
}
type newClientFunc func(conn net.Conn, opt *server.Option) (client *Client, err error)

type Call struct {
	Seq uint64
	ServiceMethod string
	Args interface{}
	Reply interface{}
	Error error
	Done chan *Call
}
func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc codec.Codec
	opt *server.Option
	sending sync.Mutex
	header codec.Header
	mu sync.Mutex
	seq uint64
	pending map[uint64]*Call
	closing bool
	shutDown bool
}
var ErrorShutdown = errors.New("connection is shutdown")

func (client *Client)Close() error  {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrorShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsValiable() bool  {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.closing && !client.shutDown
}

func (client *Client) registerCall(call *Call) (uint64, error)  {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutDown {
		return 0, ErrorShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil

}

func (client *Client) removeCall(seq uint64) *Call  {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call

}

func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutDown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

//客户端轮询接收消息
func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		if call == nil{
			err = client.cc.ReadBody(nil)
		} else if h.Error != ""{
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		} else {
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err)
}

func (client *Client)send(call *Call)  {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq,err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = call.Seq
	client.header.Error = ""

	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client)Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call  {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args: args,
		Reply: reply,
		Done: done,
	}
	client.send(call)

	return call
}
func (client *Client) Call(ctx context.Context,serviceMethod string, args, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
	case call := <- call.Done:
		return call.Error
	}
	return call.Error
}

func NewClient(conn net.Conn, option *server.Option)(*Client,error)  {
	f := codec.NewCodecFuncMap[option.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", option.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	if err := json.NewEncoder(conn).Encode(option); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), option), nil

}

func newClientCodec(cc codec.Codec, option *server.Option) *Client {
	client := &Client{
		seq:     1, // seq starts with 1, 0 means invalid call
		cc:      cc,
		opt:     option,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}
func parseOptions(opts ...*server.Option) (*server.Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return server.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = server.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = server.DefaultOption.CodecType
	}
	return opt, nil
}

func dialTimeout(f newClientFunc ,network, adress string,opts ...*server.Option) (client *Client, err error){
	opt,err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, adress, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()

	ch := make(chan ClientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- ClientResult{client: client, err: err}
	}()
	if opt.ConnectTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}
	select {
	case  <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
	case result := <-ch:
		return result.client, nil

		
	}
}


func Dial(network, adress string,opts ...*server.Option) (client *Client, err error)  {
	return dialTimeout(NewClient, network,adress,opts...)
}




