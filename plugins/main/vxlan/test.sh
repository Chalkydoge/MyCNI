ip netns add ns1

ip netns add ns2

go test -v -run TestIPAMDelegate1

go test -v -run TestIPAMDelegate2