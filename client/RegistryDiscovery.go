package client

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type RegistryDiscovery struct {
	*MultiServersDiscovery
	registryAddress string //注册中心地址
	timeout time.Duration
	lastUpdate time.Time
}
const (
	DefaultUpdateTimeout = 5 * time.Second
)

func (r *RegistryDiscovery)Update(servers []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers
	return nil

}

func (r *RegistryDiscovery)Refresh() error  {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastUpdate.Add(r.timeout).After(time.Now()) {
		return nil
	}
	log.Println("rpc registry: refresh servers from registry", r.registryAddress)
	resp,err := http.Get(r.registryAddress)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Servers"),",")
	r.servers = make([]string,0,len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			r.servers = append(r.servers, strings.TrimSpace(server))
		}
	}
	r.lastUpdate = time.Now()
	return nil
}

func (r *RegistryDiscovery)Get(mode SelectMode) (string,error)  {
	// 需要先调用 Refresh 确保服务列表没有过期。
	if err := r.Refresh(); err != nil {
		return "", err
	}
	return r.MultiServersDiscovery.Get(mode)
}
func (r *RegistryDiscovery)GetAll() ([]string,error)  {
	// 需要先调用 Refresh 确保服务列表没有过期。
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r.MultiServersDiscovery.GetAll()
}
func NewRegistryDiscovery(registryAddress string, timeout time.Duration) *RegistryDiscovery {
	if timeout == 0 {
		timeout = DefaultUpdateTimeout
	}
	d := &RegistryDiscovery{
		MultiServersDiscovery:NewMultiServersDiscovery(make([]string, 0)),
		registryAddress: registryAddress,
		timeout: timeout,
	}
	return d
}
