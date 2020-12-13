package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	timeout time.Duration
	mu sync.Mutex
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr string
	start time.Time
}

const (
	DefaultPath = "/wxy/registry"
	DefaultTimeout = 5 * time.Minute //任何注册的服务超过 5 min，即视为不可用状态
)

func NewRegistry(timeout time.Duration) *Registry  {
	return &Registry{
		timeout: timeout,
		servers: make(map[string]*ServerItem),
	}
}
var DefaultRegistry = NewRegistry(DefaultTimeout)

//添加服务实例，如果服务已经存在，则更新 start。
func (r *Registry)PutServer(addr string)  {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		s.start = time.Now()
	}
}

//返回可用的服务列表，如果存在超时的服务，则删除。
func (r *Registry)AliveServers()[]string  {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers{
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

//通过HTTP对外暴露服务
func (r *Registry) ServeHTTP(w http.ResponseWriter,req *http.Request)  {
	switch strings.ToUpper(req.Method) {
	//返回所有可用的服务列表，通过自定义字段 X-Geerpc-Servers 承载。
	case "GET":
		w.Header().Set("X-Servers",strings.Join(r.AliveServers(),","))
	//添加服务实例或发送心跳，通过自定义字段 X-Geerpc-Server 承载。
	case "POST":
		addr := req.Header.Get("X-Server")
		if addr == ""{
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.PutServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry)HandleHTTP(registryPath string)  {
	http.Handle(registryPath,r)
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultRegistry.HandleHTTP(DefaultPath)
}

func HeartBeat(registry, addr string,duration time.Duration)  {
	if duration == 0 {
		duration = DefaultTimeout - time.Duration(1) * time.Minute
	}
	var err error
	err = sendHeartBeat(registry,addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartBeat(registry,addr)
		}

	}()
}

func sendHeartBeat(registry,addr string) error  {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("Post",registry,nil)
	req.Header.Set("X-Server",addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil
}