package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

//func (t *T) MethodName(argType T1, replyType *T2) error
type MethodType struct {
	method reflect.Method
	ArgType reflect.Type
	ReplyType reflect.Type
	numCalls uint64
}

type Service struct {
	Name string
	Typ reflect.Type
	Rcvr reflect.Value
	Methods map[string]*MethodType
}

func (s *Service) registerMethods() {
	s.Methods = make(map[string]*MethodType)
	for i := 0; i < s.Typ.NumMethod(); i++ {
		method := s.Typ.Method(i)
		mType := method.Type
		//两个导出或内置类型的入参（反射时为 3 个，第 0 个是自身，类似于 python 的 self，java 中的 this）
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		//返回值有且只有 1 个，类型为 error
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1),mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.Methods[method.Name] = &MethodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.Name, method.Name)

	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *Service)Call(m *MethodType, argv,replv reflect.Value) error  {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValue := f.Call([]reflect.Value{s.Rcvr, argv, replv})
	if errInter := returnValue[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}



func (m *MethodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *MethodType) NewArgv() reflect.Value  {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *MethodType) NewReolyv() reflect.Value  {
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(),0,0))

	}
	return replyv


}

func NewService(rcvr interface{}) *Service {
	s := new(Service)
	s.Rcvr = reflect.ValueOf(rcvr)
	s.Typ = reflect.TypeOf(rcvr)
	s.Name = reflect.Indirect(s.Rcvr).Type().Name()
	if !ast.IsExported(s.Name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.Name)
	}
	s.registerMethods()
	return s

}

