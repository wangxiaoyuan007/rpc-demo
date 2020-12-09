package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
)

type Service struct {
	name string
	rcvr reflect.Value
	tpe reflect.Type
	methods map[string]reflect.Method
}
func makeService(rcvr interface{}) *Service  {
	svc := &Service{
		rcvr: reflect.ValueOf(rcvr),
		tpe: reflect.TypeOf(rcvr),
		name: reflect.Indirect(reflect.ValueOf(rcvr)).Type().Name(),
		methods: map[string]reflect.Method{},
	}
	for m := 0;m < svc.tpe.NumMethod();m++  {
		method := svc.tpe.Method(m)
		mtype := method.Type
		mName := method.Name
		if method.PkgPath != "" || //大写?
			mtype.NumIn() != 3 || //参数三个
			mtype.In(2).Kind() != reflect.Ptr || //最后一个参数不为指针
			mtype.NumOut() != 0{

		} else {
			svc.methods[mName] = method
		}
	}
	return svc
}
func (svc *Service) Dispatch(methodName string, req reqMsg) rspMsg {
	if method,ok := svc.methods[methodName];ok {
		args := reflect.New(req.argsType)

		//反序列化
		ab := bytes.NewBuffer(req.args)
		ad := gob.NewDecoder(ab)

		ad.Decode(args.Interface())

		//为返回值分配空间
		replyType := method.Type.In(2)
		replyType = replyType.Elem()
		replyV := reflect.New(replyType)

		//调用函数
		function := method.Func
		function.Call([] reflect.Value{svc.rcvr, args.Elem(), replyV})

		//序列化
		rb := new(bytes.Buffer)
		re := gob.NewEncoder(rb)
		re.EncodeValue(replyV)

		return rspMsg{true,rb.Bytes()}
	} else {
		choices := [] string{}
		for k,_ := range svc.methods {
			choices = append(choices, k)
		}
		log.Fatalf("service.dispatch:unknown service %v in %v; expecting one of %v \n",
			methodName, req.svcMeth,choices)
		return rspMsg{false,nil}
	}


}

