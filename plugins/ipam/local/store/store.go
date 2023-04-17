package store

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
)

const (
	defaultDataDir = "/var/lib/testcni"
)

// 定义了容器网络的信息
// id 表示当前的pod名称
// ifname 表示当前pod网络设备的编号
type ContainerNetInfo struct {
	ID     string `json:"id"`
	IFName string `json:"if"`
}

type Data struct {
	IPs  map[string]ContainerNetInfo `json:"ips"`
	Last string                      `json:"last"`
}

type Store struct {
	lk       *FileLock
	dir      string
	data     *Data
	filePath string // Path to local conf
}

func NewStore(dataDir, network string) (*Store, error) {
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	// 创建目录下的网络信息存储文件
	dir := filepath.Join(dataDir, network)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lk, err := NewFileLock(dir)
	if err != nil {
		return nil, err
	}

	// like: /var/lib/testcni/mynet.json
	filePath := filepath.Join(dir, network + ".json")
	
	// 初始表
	data := &Data{IPs: make(map[string]ContainerNetInfo)}

	return &Store{lk, dir, data, filePath}, nil
}

func (s *Store) LoadData() error {
	data := &Data{}
	// 读取到raw
	raw, err := os.ReadFile(s.filePath)

	if err != nil {
		if os.IsNotExist(err) {
			f, err := os.Create(s.filePath)
			if err != nil {
				return err
			}
			defer f.Close()
			// 初始化一个空文件
			_, err = f.Write([]byte("{}"))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// unmarshall
		if err := json.Unmarshal(raw, &data); err != nil {
			return err
		}
	}
	// 空的ip地址
	if data.IPs == nil {
		data.IPs = make(map[string]ContainerNetInfo)
	}

	s.data = data
	return nil
}

// 获取上次分配后的下一个可用IP
func (s *Store) Last() net.IP {
	return net.ParseIP(s.data.Last)
}

// 通过 id 获取对应容器IP
func (s *Store) GetIPByID(id string) (net.IP, bool) {
	for ip, info := range s.data.IPs {
		if info.ID == id {
			return net.ParseIP(ip), true
		}
	}
	return nil, false
}

// 加入store
func (s *Store) Add(ip net.IP, id, ifname string) error {
	if len(ip) > 0 {
		s.data.IPs[ip.String()] = ContainerNetInfo{
			ID: id,
			IFName: ifname,
		}

		s.data.Last = ip.String()
		return s.Store()
	}
	return nil
}

// 根据给定id删除使用的ip
func (s *Store) Del(id string) error {
	for ip, info := range s.data.IPs {
		if info.ID == id {
			delete(s.data.IPs, ip)
			return s.Store()
		}
	}
	return nil
}

func (s *Store) Contain(ip net.IP) bool {
	_, ok := s.data.IPs[ip.String()]
	return ok
}

func (s *Store) Store() error {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, raw, 0644)
}

func (s *Store) Close() error {
	return s.lk.Close()
}

func (s *Store) Lock() error {
	return s.lk.Lock()
}

func (s *Store) Unlock() error {
	return s.lk.Unlock()
}

func (s *Store) RLock() error {
	return s.lk.RLock()
}

func (s *Store) RUnlock() error {
	return s.lk.RUnlock()
}

