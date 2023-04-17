package store

import(
	"testing"
	"net"
)

func TestNewStore(t *testing.T) {
	store, err := NewStore("", "mynet")
	if err != nil {
		t.Log(err)
	}
	t.Log("Init local storage ok!")

	ip := net.ParseIP("10.244.0.29")
	err = store.Add(ip, "coredns-11111-22222", "eth0")
	if err != nil {
		t.Log(err)
	}

	res := store.Contain(ip)
	if res != true {
		t.Log("Res doesn't match!")
	}

	res = store.Contain(net.ParseIP("10.244.0.28"))
	if res != false {
		t.Log("Res doesn't match!")
	}

	res_ip, _ := store.GetIPByID("coredns-11111-22222")
	t.Log("Get ip " + res_ip.String())

	err = store.Del("coredns-11111-22222")
	if err != nil {
		t.Log(err)
	}
	res = store.Contain(ip)
	if res != false {
		t.Log("Res doesn't match!")
	}
}
