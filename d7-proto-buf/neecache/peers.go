package neecache

import "neecache/neecachepb"

// PeerPicker is the interface that must be implemented to locate
// the peer that owns a specific key.
type PeerPicker interface {
	// 根据传入的key选择相应的节点 PeerGetter
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter is the interface that must be implements by a peer
type PeerGetter interface {
	// 用于从对应group查找缓存值，PeerGroup就对应上述流程中的Http客户端
	//Get(group string, key string) ([]byte, error)
	Get(in *neecachepb.Request, out *neecachepb.Response) error
}
