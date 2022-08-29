package neecache

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
	"log"
	"neecache/consistenthash"
	"neecache/neecachepb"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_neecache/"
	defaultReplicas = 50
)

// HTTPPool implements PeerPicker for a pool of HTTP peers
type HTTPPool struct {
	// this peer`s base URL, e.g. "https://example.net:8000"
	self     string              // 记录自己的地址，包括主机名/IP和端口
	basePath string              // 通信前缀，默认是"/_neecache/"
	mu       sync.Mutex          // guards peers and httpGetters
	peers    *consistenthash.Map // 类型是一致性哈希算法的Map,用来根据具体的key选择节点。
	// 映射远程节点与对应的httpGetter.每一个远程节点对应一个httpGetter，因为httpGetter 与远程节点的地址 baseURL 有关
	httpGetters map[string]*httpGetter // keyed by e.g. "http://10.0.0.2:8008
}

// NewHTTPPool initializes an HTTP pool of peers.
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServerHTTP handle all http requests
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the resposne body as a proto message.
	body, err := proto.Marshal(&neecachepb.Response{
		Value: view.ByteSlice(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Set updates the pool`s list if peers.
// 实例化一致性哈希算法，并且添加了传入的节点
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		// 为每一个节点创建一个HTTP客户端 httpGetter
		p.httpGetters[peer] = &httpGetter{
			baseURL: peer + p.basePath,
		}
	}
}

// PickPeer picks a peer according to key
// PickPeer 包装了一致性哈希算法的Get的方法， 根据具体的key，选择节点，返回节点对应的HTTP客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

type httpGetter struct {
	baseURL string // baseURL 表示将要访问的远程节点的地址
}

func (h *httpGetter) Get(in *neecachepb.Request, out *neecachepb.Response) error {
	u := fmt.Sprintf(
		"%v%s/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err2 := Body.Close()
		if err2 != nil {
			err = fmt.Errorf("%v\n%v\n", err.Error(), err2.Error())
		}
	}(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: 5v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}
