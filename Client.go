package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
)

type rspMsg struct {
	ok bool
	reply [] byte
}

type reqMsg struct {
	endName interface{} //服务器名字
	svcMeth string //调用方法
	argsType reflect.Type
	args [] byte
	rsplych chan rspMsg

}

type ClientEnd struct {
	endName interface{}
	ch chan reqMsg
}

func (e * ClientEnd)Call(svcMeth string, args interface{}, reply interface{}) bool  {
	qb := new(bytes.Buffer)
	qe := gob.NewEncoder(qb)
	qe.Encode(args)
 	req := reqMsg{
 		endName: e.endName,
 		svcMeth: svcMeth,
 		argsType: reflect.TypeOf(args),
 		rsplych: make(chan rspMsg),
 		args: qb.Bytes(),
	}
	e.ch <- req
	rsp := <- req.rsplych
	if rsp.ok {
		rb := bytes.NewBuffer(rsp.reply)
		rd := gob.NewDecoder(rb)
		if  err :=rd.Decode(reply); err != nil {
			log.Fatalf("ClientEnd.Call(): decode reply :%v\n", err)
		}
		return true
	}
	return false
}